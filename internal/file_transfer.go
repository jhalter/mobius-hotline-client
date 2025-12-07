package internal

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jhalter/mobius/hotline"
)

func (m *Model) performFileTransfer(session *ServerSession, task *Task, refNum [4]byte, transferSize uint32) {
	defer func() {
		if task.Status == TaskActive {
			task.Status = TaskCompleted
			task.EndTime = time.Now()
		}
		m.program.Send(taskStatusMsg{
			taskID: task.ID,
			status: task.Status,
			err:    task.Error,
		})
	}()

	// Get server address from session's hlClient
	serverAddr := session.hlClient.Connection.RemoteAddr().String()
	host, port, _ := net.SplitHostPort(serverAddr)
	portInt, _ := strconv.Atoi(port)
	ftAddr := net.JoinHostPort(host, strconv.Itoa(portInt+1))

	m.logger.Info("Connecting to file transfer server", "addr", ftAddr, "refNum", refNum)

	conn, err := net.DialTimeout("tcp", ftAddr, 10*time.Second)
	if err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("connection failed: %w", err)
		m.logger.Error("File transfer connection failed", "err", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// Send HTXF handshake
	handshake := make([]byte, 16)
	copy(handshake[0:4], "HTXF")                              // Protocol
	copy(handshake[4:8], refNum[:])                           // Reference number
	binary.BigEndian.PutUint32(handshake[8:12], transferSize) // Data size
	// handshake[12:16] is RSVD (zeros)

	m.logger.Info("Sending HTXF handshake", "refNum", refNum, "transferSize", transferSize)
	if _, err := conn.Write(handshake); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("handshake failed: %w", err)
		m.logger.Error("Handshake failed", "err", err)
		return
	}

	// Determine local file path
	localPath := m.resolveDownloadPath(task.FileName)
	task.LocalPath = localPath

	m.logger.Info("Downloading to", "path", localPath)

	// Create directories if needed
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("mkdir failed: %w", err)
		m.logger.Error("Failed to create directory", "err", err)
		return
	}

	// Open file for writing
	file, err := os.Create(localPath)
	if err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("create file failed: %w", err)
		m.logger.Error("Failed to create file", "err", err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	// Read FlattenedFileObject header (22 bytes for the main header)
	ffoHeader := make([]byte, 24)
	if _, err := io.ReadFull(conn, ffoHeader); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("read FFO header failed: %w", err)
		m.logger.Error("Failed to read FFO header", "err", err)
		return
	}

	// Parse fork count from FlatFileHeader
	forkCount := binary.BigEndian.Uint16(ffoHeader[22:24])
	m.logger.Info("FFO header", "forkCount", forkCount)

	// Read information fork header (16 bytes)
	infoForkHeader := make([]byte, 16)
	if _, err := io.ReadFull(conn, infoForkHeader); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("read info fork header failed: %w", err)
		m.logger.Error("Failed to read info fork header", "err", err)
		return
	}

	infoForkSize := binary.BigEndian.Uint32(infoForkHeader[12:16])
	m.logger.Info("Info fork", "size", infoForkSize)

	// Read and discard information fork data
	if _, err := io.CopyN(io.Discard, conn, int64(infoForkSize)); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("read info fork failed: %w", err)
		m.logger.Error("Failed to read info fork", "err", err)
		return
	}

	// Read data fork header
	dataForkHeader := make([]byte, 16)
	if _, err := io.ReadFull(conn, dataForkHeader); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("read data fork header failed: %w", err)
		m.logger.Error("Failed to read data fork header", "err", err)
		return
	}

	dataForkSize := binary.BigEndian.Uint32(dataForkHeader[12:16])
	m.logger.Info("Data fork", "size", dataForkSize)

	// Stream data fork to file with progress tracking
	if err := m.copyWithProgress(file, conn, int64(dataForkSize), task); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("data transfer failed: %w", err)
		m.logger.Error("Data transfer failed", "err", err)
		return
	}

	// Handle resource fork if present
	if forkCount == 3 {
		m.logger.Info("Resource fork present, reading...")

		// Read resource fork header
		resForkHeader := make([]byte, 16)
		if _, err := io.ReadFull(conn, resForkHeader); err != nil {
			// Non-fatal - data fork already saved
			m.logger.Error("read resource fork header failed", "err", err)
			return
		}

		resForkSize := binary.BigEndian.Uint32(resForkHeader[12:16])
		m.logger.Info("Resource fork", "size", resForkSize)

		// Save resource fork as AppleDouble
		resPath := filepath.Join(filepath.Dir(localPath), "._"+filepath.Base(localPath))
		resFile, err := os.Create(resPath)
		if err != nil {
			m.logger.Error("create resource fork file failed", "err", err)
			return
		}
		defer func() {
			_ = resFile.Close()
		}()

		// Write AppleDouble header
		if err := m.writeAppleDoubleHeader(resFile, resForkSize); err != nil {
			m.logger.Error("write AppleDouble header failed", "err", err)
			return
		}

		// Copy resource fork data
		if _, err := io.CopyN(resFile, conn, int64(resForkSize)); err != nil {
			m.logger.Error("resource fork transfer failed", "err", err)
			return
		}

		m.logger.Info("Resource fork saved", "path", resPath)
	}

	m.logger.Info("File download completed", "path", localPath)
}

func (m *Model) copyWithProgress(dst io.Writer, src io.Reader, size int64, task *Task) error {
	buf := make([]byte, 32*1024) // 32KB buffer
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var written int64
	task.LastBytes = 0
	task.LastUpdate = time.Now()

	for written < size {
		toRead := int64(len(buf))
		if size-written < toRead {
			toRead = size - written
		}

		n, err := src.Read(buf[:toRead])
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			written += int64(n)
			task.TransferredBytes = written

			// Send progress update
			select {
			case <-ticker.C:
				m.program.Send(taskProgressMsg{
					taskID: task.ID,
					bytes:  written,
				})
			default:
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Send final progress update
	m.program.Send(taskProgressMsg{
		taskID: task.ID,
		bytes:  written,
	})

	return nil
}

func (m *Model) resolveDownloadPath(fileName string) string {
	basePath := filepath.Join(m.downloadDir, fileName)

	// Check for conflicts and auto-rename
	if _, err := os.Stat(basePath); err == nil {
		ext := filepath.Ext(fileName)
		nameWithoutExt := strings.TrimSuffix(fileName, ext)

		for i := 1; ; i++ {
			newPath := filepath.Join(m.downloadDir, fmt.Sprintf("%s (%d)%s", nameWithoutExt, i, ext))
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				return newPath
			}
		}
	}

	return basePath
}

// writeAppleDoubleHeader writes a minimal AppleDouble header for resource fork storage
func (m *Model) writeAppleDoubleHeader(w io.Writer, resourceForkSize uint32) error {
	// AppleDouble magic number
	if _, err := w.Write([]byte{0x00, 0x05, 0x16, 0x07}); err != nil {
		return err
	}

	// Version (0x00020000)
	if _, err := w.Write([]byte{0x00, 0x02, 0x00, 0x00}); err != nil {
		return err
	}

	// Filler (16 bytes of zeros)
	if _, err := w.Write(make([]byte, 16)); err != nil {
		return err
	}

	// Number of entries (1 for resource fork)
	if _, err := w.Write([]byte{0x00, 0x01}); err != nil {
		return err
	}

	// Entry descriptor for resource fork
	// Entry ID: 2 (resource fork)
	if _, err := w.Write([]byte{0x00, 0x00, 0x00, 0x02}); err != nil {
		return err
	}

	// Offset: 82 (size of header)
	if _, err := w.Write([]byte{0x00, 0x00, 0x00, 0x52}); err != nil {
		return err
	}

	// Length: resource fork size
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, resourceForkSize)
	if _, err := w.Write(sizeBytes); err != nil {
		return err
	}

	return nil
}

// performFileUpload handles the entire file upload process
func (m *Model) performFileUpload(session *ServerSession, task *Task, refNum [4]byte) {
	defer func() {
		if task.Status == TaskActive {
			task.Status = TaskCompleted
			task.EndTime = time.Now()
		}
		m.program.Send(taskStatusMsg{
			taskID: task.ID,
			status: task.Status,
			err:    task.Error,
		})
	}()

	// Open local file
	file, err := os.Open(task.LocalPath)
	if err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("open file failed: %w", err)
		m.logger.Error("Failed to open file", "err", err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("stat file failed: %w", err)
		return
	}

	// Calculate total transfer size (needed for HTXF handshake)
	infoFork := m.createInfoFork(fileInfo)

	// Check for resource fork to include in size calculation
	resPath := filepath.Join(filepath.Dir(task.LocalPath), "._"+filepath.Base(task.LocalPath))
	var resForkSize int64
	hasResourceFork := false
	if resInfo, err := os.Stat(resPath); err == nil && !resInfo.IsDir() {
		// Resource fork size = actual size - 82 byte AppleDouble header
		resForkSize = resInfo.Size() - 82
		hasResourceFork = resForkSize > 0
	}

	// Calculate total transfer size:
	// - FlatFileHeader: 24 bytes
	// - Info fork: len(infoFork) (includes header + data)
	// - Data fork header: 16 bytes
	// - Data fork data: fileInfo.Size()
	// - Resource fork header (if present): 16 bytes
	// - Resource fork data (if present): resForkSize
	totalSize := uint32(24 + len(infoFork) + 16 + int(fileInfo.Size()))
	if hasResourceFork {
		totalSize += uint32(16 + int(resForkSize))
	}

	// Connect to file transfer port (server port + 1)
	serverAddr := session.hlClient.Connection.RemoteAddr().String()
	host, port, _ := net.SplitHostPort(serverAddr)
	portInt, _ := strconv.Atoi(port)
	ftAddr := net.JoinHostPort(host, strconv.Itoa(portInt+1))

	m.logger.Info("Connecting to file transfer server", "addr", ftAddr, "refNum", refNum)

	conn, err := net.DialTimeout("tcp", ftAddr, 10*time.Second)
	if err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("connection failed: %w", err)
		m.logger.Error("File transfer connection failed", "err", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// Send HTXF handshake with total transfer size
	handshake := make([]byte, 16)
	copy(handshake[0:4], "HTXF")
	copy(handshake[4:8], refNum[:])
	binary.BigEndian.PutUint32(handshake[8:12], totalSize)
	// handshake[12:16] remains zeros (reserved)

	m.logger.Info("Sending HTXF handshake", "refNum", refNum, "totalSize", totalSize, "fileSize", fileInfo.Size())
	if _, err := conn.Write(handshake); err != nil {
		task.Status = TaskFailed
		task.Error = fmt.Errorf("handshake failed: %w", err)
		m.logger.Error("Handshake failed", "err", err)
		return
	}

	// Send FlattenedFileObject
	if err := m.sendFlattenedFileObject(conn, file, fileInfo, task); err != nil {
		task.Status = TaskFailed
		task.Error = err
		m.logger.Error("Upload failed", "err", err)
		return
	}

	m.logger.Info("File upload completed", "file", task.FileName)
}

// sendFlattenedFileObject sends the FFO structure with file data
func (m *Model) sendFlattenedFileObject(conn net.Conn, file *os.File, fileInfo os.FileInfo, task *Task) error {
	// Check for AppleDouble resource fork
	resPath := filepath.Join(filepath.Dir(task.LocalPath), "._"+filepath.Base(task.LocalPath))
	var resFile *os.File
	var resForkSize int64
	hasResourceFork := false

	if resInfo, err := os.Stat(resPath); err == nil && !resInfo.IsDir() {
		resFile, err = os.Open(resPath)
		if err == nil {
			defer func() {
				_ = resFile.Close()
			}()
			// Skip AppleDouble header (82 bytes) to get to resource fork data
			_, _ = resFile.Seek(82, 0)
			resForkSize = resInfo.Size() - 82
			hasResourceFork = resForkSize > 0
		}
	}

	// FlatFileHeader (24 bytes)
	ffoHeader := make([]byte, 24)
	// MAGIC (4 bytes): not used by modern servers
	// VERSION (2 bytes): 0x0001
	binary.BigEndian.PutUint16(ffoHeader[4:6], 1)
	// RSVD (16 bytes): zeros
	// Fork count (2 bytes): 2 or 3 (info + data + optional resource)
	forkCount := uint16(2)
	if hasResourceFork {
		forkCount = 3
	}
	binary.BigEndian.PutUint16(ffoHeader[22:24], forkCount)

	if _, err := conn.Write(ffoHeader); err != nil {
		return fmt.Errorf("write FFO header: %w", err)
	}

	// Info fork header + data
	infoFork := m.createInfoFork(fileInfo)
	if _, err := conn.Write(infoFork); err != nil {
		return fmt.Errorf("write info fork: %w", err)
	}

	// Data fork header (16 bytes)
	dataForkHeader := make([]byte, 16)
	// Fork type (4 bytes): "DATA"
	copy(dataForkHeader[0:4], "DATA")
	// Compression (2 bytes): 0 (none)
	// RSVD (2 bytes): zeros
	// RSVD (4 bytes): zeros
	// Data size (4 bytes)
	binary.BigEndian.PutUint32(dataForkHeader[12:16], uint32(fileInfo.Size()))

	if _, err := conn.Write(dataForkHeader); err != nil {
		return fmt.Errorf("write data fork header: %w", err)
	}

	// Stream file data with progress tracking
	if err := m.copyWithProgressUpload(conn, file, fileInfo.Size(), task); err != nil {
		return fmt.Errorf("data transfer: %w", err)
	}

	// Send resource fork if present
	if hasResourceFork && resFile != nil {
		resForkHeader := make([]byte, 16)
		copy(resForkHeader[0:4], "MACR") // Resource fork type
		binary.BigEndian.PutUint32(resForkHeader[12:16], uint32(resForkSize))

		if _, err := conn.Write(resForkHeader); err != nil {
			return fmt.Errorf("write resource fork header: %w", err)
		}

		// Copy resource fork data
		if _, err := io.CopyN(conn, resFile, resForkSize); err != nil {
			return fmt.Errorf("resource fork transfer: %w", err)
		}

		m.logger.Info("Resource fork uploaded", "size", resForkSize)
	}

	return nil
}

// createInfoFork creates a properly formatted Hotline information fork with
// file metadata including type codes, timestamps, and filename.
// Returns a complete byte slice containing both the fork header (16 bytes)
// and the serialized FlatFileInformationFork data.
func (m *Model) createInfoFork(fileInfo os.FileInfo) []byte {
	// Get file type and creator codes based on file extension
	ft := hotline.FileTypeFromFilename(fileInfo.Name())

	// Convert modification time to Hotline's 8-byte time format
	mTime := hotline.NewTime(fileInfo.ModTime())

	// Create the information fork using the hotline library constructor
	infoFork := hotline.NewFlatFileInformationFork(
		fileInfo.Name(),
		mTime,
		ft.TypeCode,
		ft.CreatorCode,
	)

	// Serialize the info fork using its io.Reader interface
	infoForkData, err := io.ReadAll(&infoFork)
	if err != nil {
		m.logger.Error("Failed to serialize info fork", "err", err)
		return []byte{}
	}

	// Create the fork header using the hotline library struct
	forkHeader := hotline.FlatFileForkHeader{
		ForkType: hotline.ForkTypeINFO,
		DataSize: infoFork.Size(),
	}

	// Serialize the fork header and combine with info fork data
	result := slices.Concat(
		forkHeader.ForkType[:],
		forkHeader.CompressionType[:],
		forkHeader.RSVD[:],
		forkHeader.DataSize[:],
		infoForkData,
	)

	return result
}

// copyWithProgressUpload streams data with progress tracking for uploads
func (m *Model) copyWithProgressUpload(dst io.Writer, src io.Reader, size int64, task *Task) error {
	buf := make([]byte, 32*1024) // 32KB buffer
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var written int64
	task.LastBytes = 0
	task.LastUpdate = time.Now()

	for written < size {
		toRead := int64(len(buf))
		if size-written < toRead {
			toRead = size - written
		}

		n, err := src.Read(buf[:toRead])
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			written += int64(n)
			task.TransferredBytes = written

			// Send progress update
			select {
			case <-ticker.C:
				m.program.Send(taskProgressMsg{
					taskID: task.ID,
					bytes:  written,
				})
			default:
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Send final progress update
	m.program.Send(taskProgressMsg{
		taskID: task.ID,
		bytes:  written,
	})

	return nil
}

// Helper function to format speed
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	} else {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	}
}

package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jhalter/mobius-hotline-client/internal/style"
	"github.com/jhalter/mobius/hotline"
)

// ignoredErrorMessages contains error messages from servers that should be
// silently ignored rather than displayed to the user. This helps filter out
// annoying non-standard messages from servers.
var ignoredErrorMessages = []string{
	"Uh, no.",
}

// checkTransactionError checks if a transaction has an error response and sends
// an error message to the UI if one exists.
// Returns true if an error was found, false otherwise.
func (m *Model) checkTransactionError(t *hotline.Transaction) bool {
	if t.ErrorCode != [4]byte{0, 0, 0, 0} {
		errorText := string(t.GetField(hotline.FieldError).Data)

		// Check if this error should be ignored
		for _, ignored := range ignoredErrorMessages {
			if errorText == ignored {
				return true // Error exists but is ignored
			}
		}

		m.program.Send(errorMsg{text: errorText})
		return true
	}
	return false
}

func (m *Model) HandleKeepAlive(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	return res, err
}

func (m *Model) HandleTranServerMsg(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	// Play sound for private message
	if m.soundPlayer != nil {
		m.soundPlayer.PlayAsync(SoundServerMsg)
	}

	now := time.Now().Format(time.RFC850)

	msg := strings.ReplaceAll(string(t.GetField(hotline.FieldData).Data), "\r", "\n")
	from := string(t.GetField(hotline.FieldUserName).Data)
	userIDField := t.GetField(hotline.FieldUserID)
	var userID [2]byte
	if len(userIDField.Data) >= 2 {
		copy(userID[:], userIDField.Data[:2])
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(serverMsgMsg{from: from, userID: userID, text: msg, time: now})

	return res, err
}

func (m *Model) HandleGetFileNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	var files []hotline.FileNameWithInfo
	for _, f := range t.Fields {
		var fn hotline.FileNameWithInfo
		_, err = fn.Write(f.Data)
		if err != nil {
			continue
		}
		files = append(files, fn)
	}

	m.program.Send(filesMsg{files: files})

	return res, err
}

func (m *Model) TranGetMsgs(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	messageBoardText := string(t.GetField(hotline.FieldData).Data)
	messageBoardText = strings.ReplaceAll(messageBoardText, "\r", "\n")

	// Send message to Bubble Tea program to update UI
	m.program.Send(messageBoardMsg{text: messageBoardText})

	return res, err
}

func (m *Model) HandleNotifyChangeUser(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	sessionID := getSessionIDFromContext(ctx)
	session := m.getSessionByID(sessionID)
	if session == nil {
		return nil, nil
	}

	newUser := hotline.User{
		ID:    [2]byte(t.GetField(hotline.FieldUserID).Data),
		Name:  string(t.GetField(hotline.FieldUserName).Data),
		Icon:  t.GetField(hotline.FieldUserIconID).Data,
		Flags: t.GetField(hotline.FieldUserFlags).Data,
	}

	var oldName string
	var newUserList []hotline.User
	updatedUser := false

	for _, u := range session.userList {
		if newUser.ID == u.ID {
			oldName = u.Name
			if u.Name != newUser.Name {
				m.program.Send(chatMsg{text: fmt.Sprintf(" <<< %s is now known as %s >>>", oldName, newUser.Name)})
			}
			u = newUser
			updatedUser = true
		}
		newUserList = append(newUserList, u)
	}

	if !updatedUser {
		newUserList = append(newUserList, newUser)
		// Play sound for user joining
		if m.soundPlayer != nil {
			m.soundPlayer.PlayAsync(SoundUserJoin)
		}
		// Send join message to chat
		joinStyle := lipgloss.NewStyle().Bold(true).Foreground(style.CurrentTheme.TextMuted)
		m.program.Send(chatMsg{text: joinStyle.Render(fmt.Sprintf("→ %s joined", newUser.Name))})
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(userListMsg{users: newUserList})

	return res, err
}

func (m *Model) HandleNotifyDeleteUser(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	sessionID := getSessionIDFromContext(ctx)
	session := m.getSessionByID(sessionID)
	if session == nil {
		return nil, nil
	}

	exitUser := t.GetField(hotline.FieldUserID).Data

	// Find the username before removing
	var leavingUsername string
	var newUserList []hotline.User
	for _, u := range session.userList {
		if !bytes.Equal(exitUser, u.ID[:]) {
			newUserList = append(newUserList, u)
		} else {
			leavingUsername = u.Name
		}
	}

	// Play sound for user leaving
	if m.soundPlayer != nil {
		m.soundPlayer.PlayAsync(SoundUserLeave)
	}

	// Send leave message to chat
	if leavingUsername != "" {
		leaveStyle := lipgloss.NewStyle().Bold(true).Foreground(style.CurrentTheme.TextMuted)
		m.program.Send(chatMsg{text: leaveStyle.Render(fmt.Sprintf("← %s left", leavingUsername))})
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(userListMsg{users: newUserList})

	return res, err
}

func (m *Model) HandleClientGetUserNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	var users []hotline.User
	for _, field := range t.Fields {
		if field.Type == hotline.FieldUsernameWithInfo {
			var user hotline.User
			if _, err := user.Write(field.Data); err != nil {
				return res, fmt.Errorf("unable to read user data: %w", err)
			}
			users = append(users, user)
		}
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(userListMsg{users: users})

	return res, err
}

func (m *Model) HandleClientChatMsg(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	// Play sound for chat message
	if m.soundPlayer != nil {
		m.soundPlayer.PlayAsync(SoundChatMsg)
	}

	// Terminal bell (independent of sounds)
	if m.prefs.EnableBell {
		fmt.Print("\a")
	}

	chatText := string(t.GetField(hotline.FieldData).Data)
	chatText = strings.ReplaceAll(chatText, "\r", "")
	// Send message to Bubble Tea program to update UI
	m.program.Send(chatMsg{text: chatText})

	return res, err
}

func (m *Model) HandleClientTranUserAccess(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	sessionID := getSessionIDFromContext(ctx)
	session := m.getSessionByID(sessionID)
	if session == nil {
		return nil, nil
	}

	copy(session.userAccess[:], t.GetField(hotline.FieldUserAccess).Data)
	m.logger.Debug("Permissions", "bits", fmt.Sprintf("%b", session.userAccess))

	// Enable/disable keybinding depending on access.
	if session.serverScreen != nil {
		session.serverScreen.SetUserAccess(session.userAccess)
	}
	if session.messageBoardScreen != nil {
		session.messageBoardScreen.SetUserAccess(session.userAccess)
	}

	return res, err
}

func (m *Model) HandleClientTranShowAgreement(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	agreement := string(t.GetField(hotline.FieldData).Data)
	agreement = strings.ReplaceAll(agreement, "\r", "\n")

	// Show agreement modal with Agree/Disagree options
	m.program.Send(agreementMsg{text: agreement})

	return res, err
}

func (m *Model) HandleClientTranLogin(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, errors.New("login error")
	}

	// Send server connected message with the name to display
	m.program.Send(serverConnectedMsg{name: m.pendingServerName})

	if err := c.Send(hotline.NewTransaction(hotline.TranGetUserNameList, [2]byte{})); err != nil {
		m.logger.Error("err", "err", err)
	}

	return res, err
}

func (m *Model) HandleDownloadFile(ctx context.Context, c *hotline.Client, t *hotline.Transaction) ([]hotline.Transaction, error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	refNumField := t.GetField(hotline.FieldRefNum)
	transferSizeField := t.GetField(hotline.FieldTransferSize)
	fileSizeField := t.GetField(hotline.FieldFileSize)

	if refNumField == nil || len(refNumField.Data) < 4 ||
		transferSizeField == nil || len(transferSizeField.Data) < 4 ||
		fileSizeField == nil || len(fileSizeField.Data) < 4 {
		return nil, fmt.Errorf("missing required fields in download response")
	}

	transferSize := binary.BigEndian.Uint32(transferSizeField.Data)
	fileSize := binary.BigEndian.Uint32(fileSizeField.Data)

	var refNumBytes [4]byte
	copy(refNumBytes[:], refNumField.Data)

	m.program.Send(downloadReplyMsg{
		txID:         t.ID,
		refNum:       refNumBytes,
		transferSize: transferSize,
		fileSize:     fileSize,
	})

	return nil, nil
}

func (m *Model) HandleUploadFile(ctx context.Context, c *hotline.Client, t *hotline.Transaction) ([]hotline.Transaction, error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	refNum := t.GetField(hotline.FieldRefNum).Data

	var refNumBytes [4]byte
	copy(refNumBytes[:], refNum)

	m.program.Send(uploadReplyMsg{
		txID:   t.ID,
		refNum: refNumBytes,
	})
	m.logger.Info("Upload transaction ID", "id", t.ID)

	return nil, nil
}

func (m *Model) HandleGetFileInfo(ctx context.Context, c *hotline.Client, t *hotline.Transaction) ([]hotline.Transaction, error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	// Extract file info fields from response
	msg := fileInfoMsg{
		fileName: string(t.GetField(hotline.FieldFileName).Data),
	}

	// Extract type and creator strings with nil checks
	if typeField := t.GetField(hotline.FieldFileTypeString); typeField != nil {
		msg.fileTypeString = string(typeField.Data)
	}
	if creatorField := t.GetField(hotline.FieldFileCreatorString); creatorField != nil {
		msg.fileCreatorString = string(creatorField.Data)
	}

	// Extract fixed-size fields
	fileTypeData := t.GetField(hotline.FieldFileType).Data
	if len(fileTypeData) >= 4 {
		copy(msg.fileType[:], fileTypeData[:4])
	}

	createDateField := t.GetField(hotline.FieldFileCreateDate)
	if createDateField != nil && len(createDateField.Data) >= 8 {
		copy(msg.createDate[:], createDateField.Data[:8])
	}

	modifyDateField := t.GetField(hotline.FieldFileModifyDate)
	if modifyDateField != nil && len(modifyDateField.Data) >= 8 {
		copy(msg.modifyDate[:], modifyDateField.Data[:8])
	}

	// Extract optional fields
	commentField := t.GetField(hotline.FieldFileComment)
	if commentField != nil && len(commentField.Data) > 0 {
		msg.comment = string(commentField.Data)
	}

	fileSizeField := t.GetField(hotline.FieldFileSize)
	if fileSizeField != nil && len(fileSizeField.Data) >= 4 {
		msg.fileSize = binary.BigEndian.Uint32(fileSizeField.Data)
		msg.hasFileSize = true
	}

	m.program.Send(msg)
	return nil, nil
}

func (m *Model) HandleGetNewsCatNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	var categories []newsItem

	// Parse category list data from transaction fields
	for _, f := range t.Fields {
		m.logger.Debug("type", "f.Type", f.Type, "catlist", f.Type == hotline.FieldNewsCatListData15)
		if f.Type == hotline.FieldNewsCatListData15 {
			var catData hotline.NewsCategoryListData15
			_, err = catData.Write(f.Data)
			if err != nil {
				m.logger.Error("Error parsing category data", "err", err)
				continue
			}

			// Determine if this is a bundle or category
			isBundle := bytes.Equal(catData.Type[:], hotline.NewsBundle[:])

			categories = append(categories, newsItem{
				name:     catData.Name,
				isBundle: isBundle,
			})
		}
	}

	// Send categories to UI
	m.program.Send(newsCategoriesMsg{categories: categories})

	return res, err
}

func (m *Model) HandleGetNewsArtNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	var articles []newsArticleItem

	// Get the NewsArtListData field
	artListField := t.GetField(hotline.FieldNewsArtListData)

	// Parse the NewsArtListData using the Write method
	var artListData hotline.NewsArtListData
	_, err = artListData.Write(artListField.Data)
	if err != nil {
		m.logger.Error("Error parsing article list data", "err", err)
		return res, err
	}

	// Parse individual articles from the NewsArtList payload
	data := artListData.NewsArtList
	offset := 0
	for i := 0; i < artListData.Count && offset < len(data); i++ {
		if offset+4 > len(data) {
			break
		}

		// Read article ID (4 bytes)
		articleID := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Read timestamp (8 bytes), parent ID (4 bytes), then skip flags (4 bytes) and flavor count (2 bytes)
		if offset+18 > len(data) {
			break
		}
		timestamp := [8]byte{}
		copy(timestamp[:], data[offset:offset+8])
		offset += 8

		// Read parent ID (4 bytes)
		parentID := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		offset += 4 + 2 // skip flags + flavor count

		// Read title (1 byte length + data)
		if offset+1 > len(data) {
			break
		}
		titleLen := int(data[offset])
		offset++
		if offset+titleLen > len(data) {
			break
		}
		title := string(data[offset : offset+titleLen])
		offset += titleLen

		// Read poster (1 byte length + data)
		if offset+1 > len(data) {
			break
		}
		posterLen := int(data[offset])
		offset++
		if offset+posterLen > len(data) {
			break
		}
		poster := string(data[offset : offset+posterLen])
		offset += posterLen

		// Skip flavor text (1 byte length + data) and article size (2 bytes)
		if offset+1 > len(data) {
			break
		}
		flavorLen := int(data[offset])
		offset++
		if offset+flavorLen+2 > len(data) {
			break
		}
		offset += flavorLen + 2 // skip flavor + article size

		articles = append(articles, newsArticleItem{
			id:          articleID,
			title:       title,
			poster:      poster,
			date:        timestamp,
			parentID:    parentID,
			depth:       0,
			isExpanded:  false,
			hasChildren: false,
		})
	}

	// Send articles to UI
	m.program.Send(newsArticlesMsg{articles: articles})

	return res, err
}

func (m *Model) HandleGetNewsArtData(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	// Parse article data from transaction fields
	var article hotline.NewsArtData
	article.Title = string(t.GetField(hotline.FieldNewsArtTitle).Data)
	article.Poster = string(t.GetField(hotline.FieldNewsArtPoster).Data)
	article.Data = string(t.GetField(hotline.FieldNewsArtData).Data)
	article.Data = strings.ReplaceAll(article.Data, "\r", "\n")

	dateField := t.GetField(hotline.FieldNewsArtDate)
	if len(dateField.Data) >= 8 {
		copy(article.Date[:], dateField.Data[:8])
	}

	// Send article to UI
	m.program.Send(newsArticleDataMsg{article: article})

	return res, err
}

func (m *Model) HandlePostNewsArt(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		m.logger.Error("Error posting news article")
		return nil, nil
	}

	m.logger.Info("News article posted successfully")
	return res, err
}

func (m *Model) HandleNewNewsFldr(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		m.logger.Error("Error creating news bundle")
		return nil, nil
	}

	m.logger.Info("News bundle created successfully")
	return res, err
}

func (m *Model) HandleNewNewsCat(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		m.logger.Error("Error creating news category")
		return nil, nil
	}

	m.logger.Info("News category created successfully")
	return res, err
}

func (m *Model) HandleNewMsg(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.soundPlayer != nil {
		m.soundPlayer.PlayAsync(SoundNewNews)
	}
	return res, err
}

func (m *Model) HandleTranAgreed(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if m.checkTransactionError(t) {
		return nil, nil
	}

	m.NavigateTo(ScreenServerUI)
	if err := c.Send(hotline.NewTransaction(hotline.TranGetUserNameList, [2]byte{})); err != nil {
		m.logger.Error("err", "err", err)
	}

	return res, err
}

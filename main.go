package main

import (
	"flag"
	"fmt"
	"github.com/jhalter/mobius/hotline"
	"github.com/rivo/tview"
	"log/slog"
	"mobius-hotline-client/ui"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// Values swapped in by go-releaser at build time
var (
	version = "dev"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

	configDir := flag.String("config", defaultConfigPath(), "Path to config root")
	showVersion := flag.Bool("version", false, "print version and exit")
	logLevel := flag.String("log-level", "info", "Log level")
	logFile := flag.String("log-file", "", "output logs to file")

	flag.Parse()

	if *showVersion {
		fmt.Printf("v%s\n", version)
		os.Exit(0)
	}

	// init DebugBuffer
	db := &ui.DebugBuffer{TextView: tview.NewTextView()}

	// Add file logger if optional log-file flag was passed
	if *logFile != "" {
		f, err := os.OpenFile(*logFile,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		defer func() { _ = f.Close() }()
		if err != nil {
			panic(err)
		}
	}

	logger := slog.New(slog.NewTextHandler(db, &slog.HandlerOptions{Level: logLevels[*logLevel]}))
	logger.Info("Started Mobius client", "Version", version)

	go func() {
		sig := <-sigChan
		logger.Info("Stopping client", "signal", sig.String())
		//cancelRoot()
	}()

	client := ui.NewUIClient(*configDir, logger, db)

	// Register transaction handlers for transaction types that we should act on.
	client.HLClient.HandleFunc(hotline.TranChatMsg, client.HandleClientChatMsg)
	client.HLClient.HandleFunc(hotline.TranLogin, client.HandleClientTranLogin)
	client.HLClient.HandleFunc(hotline.TranShowAgreement, client.HandleClientTranShowAgreement)
	client.HLClient.HandleFunc(hotline.TranUserAccess, client.HandleClientTranUserAccess)
	client.HLClient.HandleFunc(hotline.TranGetUserNameList, client.HandleClientGetUserNameList)
	client.HLClient.HandleFunc(hotline.TranNotifyChangeUser, client.HandleNotifyChangeUser)
	client.HLClient.HandleFunc(hotline.TranNotifyChatDeleteUser, client.HandleNotifyDeleteUser)
	client.HLClient.HandleFunc(hotline.TranGetMsgs, client.TranGetMsgs)
	client.HLClient.HandleFunc(hotline.TranGetFileNameList, client.HandleGetFileNameList)
	client.HLClient.HandleFunc(hotline.TranServerMsg, client.HandleTranServerMsg)
	client.HLClient.HandleFunc(hotline.TranKeepAlive, client.HandleKeepAlive)

	client.Start()
}

func defaultConfigPath() (cfgPath string) {
	switch runtime.GOOS {
	case "windows":
		cfgPath = "mobius-client-config.yaml"
	case "darwin":
		if _, err := os.Stat("/usr/local/etc/mobius-client-config.yaml"); err == nil {
			cfgPath = "/usr/local/etc/mobius-client-config.yaml"
		} else if _, err := os.Stat("/opt/homebrew/etc/mobius-client-config.yaml"); err == nil {
			cfgPath = "/opt/homebrew/etc/mobius-client-config.yaml"
		}
	case "linux":
		cfgPath = "/usr/local/etc/mobius-client-config.yaml"
	default:
		fmt.Printf("unsupported OS")
	}

	return cfgPath
}

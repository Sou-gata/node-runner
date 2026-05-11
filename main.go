package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"gopkg.in/ini.v1"
)

// ===============================
// EMBED TRAY ICON
// ===============================

//go:embed icon.ico
var iconData []byte

// ===============================
// CONFIG STRUCT
// ===============================

type Config struct {
	ScriptPath string
	NodePath   string
	EnableLog  bool
	LogFile    string
	AutoStart  bool
}

// ===============================
// GLOBALS
// ===============================

var (
	cmd         *exec.Cmd
	config      Config
	mutexHandle windows.Handle
)

// ===============================
// MAIN
// ===============================

func main() {
	enforceSingleInstance()
	systray.Run(onReady, onExit)
}

// ===============================
// SINGLE INSTANCE CHECK
// ===============================

func enforceSingleInstance() {
	mutexName, err := windows.UTF16PtrFromString("NodeRunner_SingleInstance_Mutex_Lock")
	if err != nil {
		return
	}

	mutexHandle, err = windows.CreateMutex(nil, false, mutexName)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			os.Exit(0)
		}
	}
}

// ===============================
// ON READY
// ===============================

func onReady() {

	createRequiredFolders()
	systray.SetIcon(iconData)
	systray.SetTitle("Node Runner")
	systray.SetTooltip("Node.js Background Runner")

	loadConfig()

	if config.AutoStart {
		registerStartup()
	}

	err := startNodeProcess()
	if err != nil {
		logError(err.Error())
	}

	// ===============================
	// MENU ITEMS
	// ===============================

	mRestart := systray.AddMenuItem(
		"Restart Service",
		"Restart Node.js service",
	)

	mStop := systray.AddMenuItem(
		"Stop Service",
		"Stop Node.js service",
	)

	mStart := systray.AddMenuItem(
		"Start Service",
		"Start Node.js service",
	)

	systray.AddSeparator()

	// Added Auto Start Checkbox Menu Item
	mAutoStart := systray.AddMenuItemCheckbox(
		"Run on Startup",
		"Start application automatically with Windows",
		config.AutoStart,
	)

	systray.AddSeparator()

	mOpenLog := systray.AddMenuItem(
		"Open Log",
		"Open application log",
	)

	mOpenConfig := systray.AddMenuItem(
		"Open Config",
		"Open config.ini",
	)

	systray.AddSeparator()

	mExit := systray.AddMenuItem(
		"Exit",
		"Exit application",
	)

	// ===============================
	// MENU EVENTS
	// ===============================

	go func() {
		for {
			select {

			case <-mRestart.ClickedCh:
				restartProcess()

			case <-mStop.ClickedCh:
				stopProcess()

			case <-mStart.ClickedCh:
				err := startNodeProcess()
				if err != nil {
					logError(err.Error())
				}

			// Handle Auto Start Toggle
			case <-mAutoStart.ClickedCh:
				if mAutoStart.Checked() {
					mAutoStart.Uncheck()
					config.AutoStart = false
					unregisterStartup()
				} else {
					mAutoStart.Check()
					config.AutoStart = true
					registerStartup()
				}
				saveConfig() // Save the new setting to config.ini

			case <-mOpenLog.ClickedCh:
				openFile(config.LogFile)

			case <-mOpenConfig.ClickedCh:
				openFile("config.ini")

			case <-mExit.ClickedCh:
				stopProcess()
				systray.Quit()
				return
			}
		}
	}()
}

// ===============================
// ON EXIT
// ===============================

func onExit() {
	stopProcess()
	if mutexHandle != 0 {
		windows.CloseHandle(mutexHandle)
	}
}

// ===============================
// CREATE REQUIRED FOLDERS
// ===============================

func createRequiredFolders() {
	os.MkdirAll("logs", os.ModePerm)
}

// ===============================
// LOAD & SAVE CONFIG
// ===============================

func loadConfig() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		logError(err.Error())
		return
	}

	config.ScriptPath = strings.TrimSpace(cfg.Section("APP").Key("ScriptPath").String())
	config.NodePath = strings.TrimSpace(cfg.Section("APP").Key("NodePath").String())
	config.EnableLog = cfg.Section("APP").Key("EnableLogging").MustBool(true)
	config.LogFile = cfg.Section("APP").Key("LogFile").MustString("logs/app.log")
	config.AutoStart = cfg.Section("APP").Key("AutoStart").MustBool(true)

	if config.NodePath == "" {
		config.NodePath = "node"
	}
}

// Added function to save config back to file
func saveConfig() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		// If file doesn't exist for some reason, create an empty one
		cfg = ini.Empty()
	}

	// Update the AutoStart key
	cfg.Section("APP").Key("AutoStart").SetValue(strconv.FormatBool(config.AutoStart))

	err = cfg.SaveTo("config.ini")
	if err != nil {
		logError("Failed to save config: " + err.Error())
	}
}

// ===============================
// START & STOP PROCESS
// ===============================

func startNodeProcess() error {
	if cmd != nil && cmd.Process != nil {
		return nil
	}

	if config.ScriptPath == "" {
		return fmt.Errorf("ScriptPath missing in config.ini")
	}

	absScriptPath, err := filepath.Abs(config.ScriptPath)
	if err != nil {
		return err
	}

	cmd = exec.Command(config.NodePath, absScriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if config.EnableLog {
		logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err == nil {
			writer := io.MultiWriter(logFile)
			cmd.Stdout = writer
			cmd.Stderr = writer
		}
	}

	err = cmd.Start()
	if err != nil {
		cmd = nil
		return err
	}

	go func() {
		cmd.Wait()
		cmd = nil
	}()

	return nil
}

func stopProcess() {
	if cmd != nil && cmd.Process != nil {
		err := cmd.Process.Kill()
		if err != nil {
			logError(err.Error())
		}
		cmd = nil
	}
}

func restartProcess() {
	stopProcess()
	err := startNodeProcess()
	if err != nil {
		logError(err.Error())
	}
}

// ===============================
// WINDOWS STARTUP REGISTRY
// ===============================

func registerStartup() {
	exePath, err := os.Executable()
	if err != nil {
		logError(err.Error())
		return
	}

	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE|registry.QUERY_VALUE,
	)

	if err != nil {
		logError(err.Error())
		return
	}
	defer key.Close()

	appName := "Node Runner"
	currentValue, _, _ := key.GetStringValue(appName)

	if currentValue == exePath {
		return
	}

	err = key.SetStringValue(appName, exePath)
	if err != nil {
		logError(err.Error())
	}
}

// Added function to remove app from registry
func unregisterStartup() {
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE,
	)

	if err != nil {
		logError("Failed to open registry for removal: " + err.Error())
		return
	}
	defer key.Close()

	appName := "Node Runner"
	err = key.DeleteValue(appName)

	// Ignore ErrNotExist because it means it's already removed
	if err != nil && err != registry.ErrNotExist {
		logError("Failed to delete registry key: " + err.Error())
	}
}

// ===============================
// UTILS
// ===============================

func logError(msg string) {
	f, err := os.OpenFile("logs/error.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(msg + "\n")
}

func openFile(path string) {
	exec.Command("notepad", path).Start()
}
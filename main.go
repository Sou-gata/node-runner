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

// EMBED TRAY ICON

//go:embed icon.ico
var iconData []byte

// CONFIG STRUCT

type ScriptConfig struct {
	Path    string
	Name    string
	LogFile string
}

type Config struct {
	Scripts   []ScriptConfig
	NodePath  string
	EnableLog bool
	LogFile   string
	AutoStart bool
}

type ServerInstance struct {
	Config    ScriptConfig
	Cmd       *exec.Cmd
	MenuItem  *systray.MenuItem
	MStart    *systray.MenuItem
	MStop     *systray.MenuItem
	MRestart  *systray.MenuItem
	MOpenLog  *systray.MenuItem
	IsRunning bool
}

// GLOBALS

var (
	servers     []*ServerInstance
	config      Config
	mutexHandle windows.Handle
)

// MAIN

func main() {
	enforceSingleInstance()
	systray.Run(onReady, onExit)
}

// SINGLE INSTANCE CHECK

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

// ON READY

func onReady() {

	createRequiredFolders()
	systray.SetIcon(iconData)
	systray.SetTitle("Node Runner")
	systray.SetTooltip("Node.js Background Runner")

	loadConfig()

	if config.AutoStart {
		registerStartup()
	}

	// MENU ITEMS

	mRestartAll := systray.AddMenuItem(
		"Restart All Services",
		"Restart all running Node.js services",
	)

	mStopAll := systray.AddMenuItem(
		"Stop All Services",
		"Stop all running Node.js services",
	)

	mStartAll := systray.AddMenuItem(
		"Start All Services",
		"Start all configured Node.js services",
	)

	systray.AddSeparator()

	// Create submenus for each individual server
	for _, sc := range config.Scripts {
		inst := &ServerInstance{
			Config: sc,
		}

		inst.MenuItem = systray.AddMenuItem("🔴 "+sc.Name, "Server: "+sc.Name)
		inst.MStart = inst.MenuItem.AddSubMenuItem("Start", "Start "+sc.Name)
		inst.MStop = inst.MenuItem.AddSubMenuItem("Stop", "Stop "+sc.Name)
		inst.MRestart = inst.MenuItem.AddSubMenuItem("Restart", "Restart "+sc.Name)
		inst.MOpenLog = inst.MenuItem.AddSubMenuItem("Open Log", "View log for "+sc.Name)

		servers = append(servers, inst)

		// Start background listener for this instance's menu clicks
		go func(srv *ServerInstance) {
			for {
				select {
				case <-srv.MStart.ClickedCh:
					srv.Start()
				case <-srv.MStop.ClickedCh:
					srv.Stop()
				case <-srv.MRestart.ClickedCh:
					srv.Restart()
				case <-srv.MOpenLog.ClickedCh:
					openFile(srv.Config.LogFile)
				}
			}
		}(inst)
	}

	// Initially start all configured servers
	for _, srv := range servers {
		srv.Start()
	}

	systray.AddSeparator()

	// Added Auto Start Checkbox Menu Item
	mAutoStart := systray.AddMenuItemCheckbox(
		"Run on Startup",
		"Start application automatically with Windows",
		config.AutoStart,
	)

	systray.AddSeparator()

	mOpenLogFolder := systray.AddMenuItem(
		"Open Logs Folder",
		"Open directory containing all server logs",
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

	// GLOBAL MENU EVENTS

	go func() {
		for {
			select {

			case <-mRestartAll.ClickedCh:
				for _, srv := range servers {
					srv.Restart()
				}

			case <-mStopAll.ClickedCh:
				for _, srv := range servers {
					srv.Stop()
				}

			case <-mStartAll.ClickedCh:
				for _, srv := range servers {
					srv.Start()
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

			case <-mOpenLogFolder.ClickedCh:
				exec.Command("explorer", "logs").Start()

			case <-mOpenConfig.ClickedCh:
				openFile("config.ini")

			case <-mExit.ClickedCh:
				for _, srv := range servers {
					srv.Stop()
				}
				systray.Quit()
				return
			}
		}
	}()
}

// ON EXIT

func onExit() {
	for _, srv := range servers {
		srv.Stop()
	}
	if mutexHandle != 0 {
		windows.CloseHandle(mutexHandle)
	}
}

// CREATE REQUIRED FOLDERS

func createRequiredFolders() {
	os.MkdirAll("logs", os.ModePerm)
}

// LOAD & SAVE CONFIG

func loadConfig() {
	cfg, err := ini.LoadSources(ini.LoadOptions{AllowShadows: true}, "config.ini")
	if err != nil {
		logError(err.Error())
		return
	}

	var scripts []ScriptConfig
	if cfg.Section("APP").HasKey("ScriptPath") {
		for _, val := range cfg.Section("APP").Key("ScriptPath").ValueWithShadows() {
			// Split by comma in case multiple are defined inline
			parts := strings.Split(val, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}

				// Check for optional name and log file using pipe '|'
				subParts := strings.SplitN(part, "|", 3)
				path := strings.TrimSpace(subParts[0])
				name := ""
				logFile := ""
				if len(subParts) > 1 {
					name = strings.TrimSpace(subParts[1])
				}
				if len(subParts) > 2 {
					logFile = strings.TrimSpace(subParts[2])
				}

				if path != "" {
					if name == "" {
						base := filepath.Base(path)
						dir := filepath.Base(filepath.Dir(path))
						if dir == "." || dir == "\\" || dir == "/" {
							name = base
						} else {
							name = dir + "/" + base
						}
					}

					if logFile == "" {
						// Create unique sanitized log file name for this server
						sanitized := strings.ReplaceAll(name, "/", "_")
						sanitized = strings.ReplaceAll(sanitized, "\\", "_")
						sanitized = strings.ReplaceAll(sanitized, " ", "_")
						sanitized = strings.ReplaceAll(sanitized, ":", "_")
						logFile = filepath.Join("logs", sanitized+".log")
					}

					scripts = append(scripts, ScriptConfig{Path: path, Name: name, LogFile: logFile})
				}
			}
		}
	}
	config.Scripts = scripts

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
	cfg, err := ini.LoadSources(ini.LoadOptions{AllowShadows: true}, "config.ini")
	if err != nil {
		cfg = ini.Empty()
	}

	cfg.Section("APP").Key("AutoStart").SetValue(strconv.FormatBool(config.AutoStart))

	err = cfg.SaveTo("config.ini")
	if err != nil {
		logError("Failed to save config: " + err.Error())
	}
}

// SERVER INSTANCE LOGIC

func (s *ServerInstance) Start() {
	// Check if already running
	if s.Cmd != nil && s.Cmd.Process != nil && s.Cmd.ProcessState == nil {
		return
	}

	absPath, err := filepath.Abs(s.Config.Path)
	if err != nil {
		logError(fmt.Sprintf("Failed to resolve absolute path for %s: %v", s.Config.Path, err))
		return
	}

	s.Cmd = exec.Command(config.NodePath, absPath)
	s.Cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if config.EnableLog {
		os.MkdirAll(filepath.Dir(s.Config.LogFile), os.ModePerm)
		logFile, err := os.OpenFile(s.Config.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err == nil {
			writer := io.MultiWriter(logFile)
			s.Cmd.Stdout = writer
			s.Cmd.Stderr = writer
		} else {
			logError(fmt.Sprintf("Failed to open log file %s for server %s: %v", s.Config.LogFile, s.Config.Name, err))
		}
	}

	err = s.Cmd.Start()
	if err != nil {
		logError(fmt.Sprintf("Failed to start server %s: %v", s.Config.Name, err))
		s.updateState(false)
		return
	}

	s.updateState(true)

	go func(cmdObj *exec.Cmd) {
		cmdObj.Wait()
		s.updateState(false)
	}(s.Cmd)
}

func (s *ServerInstance) Stop() {
	if s.Cmd != nil && s.Cmd.Process != nil {
		if s.Cmd.ProcessState == nil {
			s.Cmd.Process.Kill()
		}
	}
	s.updateState(false)
}

func (s *ServerInstance) Restart() {
	s.Stop()
	s.Start()
}

func (s *ServerInstance) updateState(running bool) {
	s.IsRunning = running
	if s.MenuItem != nil {
		if running {
			s.MenuItem.SetTitle("🟢 " + s.Config.Name)
			if s.MStart != nil {
				s.MStart.Disable()
			}
			if s.MStop != nil {
				s.MStop.Enable()
			}
		} else {
			s.MenuItem.SetTitle("🔴 " + s.Config.Name)
			if s.MStart != nil {
				s.MStart.Enable()
			}
			if s.MStop != nil {
				s.MStop.Disable()
			}
		}
	}
}

// WINDOWS STARTUP REGISTRY

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

// UTILS

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
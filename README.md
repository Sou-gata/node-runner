# NodeRunner

> **Developed by Sougata Talukdar**

NodeRunner is a lightweight, system-tray based Windows background service runner built in Go. It is specifically designed to quietly run multiple Node.js applications in the background simultaneously while providing an intuitive, premium graphical interface from the system tray to monitor and manage each service independently.

## Features

* **Multi-Server Management**: Configure and run multiple distinct Node.js applications concurrently.
* **Dynamic Status Submenus**: Each server has its own expandable submenu displaying real-time operational status indicators (`🟢` Running / `🔴` Stopped).
* **Independent Controls**: Start, stop, restart, or inspect logs for each running server individually without affecting other active services.
* **Global Controls**: Top-level actions allow starting, stopping, or restarting all configured services simultaneously.
* **Silent Execution**: Runs processes in the background without visible console windows.
* **Smart Separate Logging**: Automatically routes stdout/stderr for each server into dedicated, sanitized log files to prevent output interleaving.
* **Automatic Recovery**: Configurable startup retry count and backoff delay ensure resilient service initialization if a server fails to start or crashes unexpectedly.
* **Custom Configuration**: Highly flexible `config.ini` syntax supporting inline arrays, custom server display names, and dedicated per-service log paths.
* **Auto-Start Integration**: Can be toggled directly from the tray menu to launch automatically with Windows.

## System Tray Menu

Right-clicking the NodeRunner icon in the system tray reveals the following options:

### Global Commands
* **Restart All Services**: Restarts all currently running Node.js background processes.
* **Stop All Services**: Safely terminates all running Node.js background processes.
* **Start All Services**: Launches all configured servers that are not currently active.

### Individual Server Submenus (e.g., `🟢 APK Server` or `🔴 Auth API`)
* **Start**: Launches this specific server instance.
* **Stop**: Terminates this specific server instance.
* **Restart**: Restarts this specific server instance.
* **Open Log**: Opens this server's dedicated log file instantly in Notepad.

### Application Settings & Utilities
* **Run on Startup**: Toggles whether NodeRunner automatically starts with Windows.
* **Open Logs Folder**: Opens Windows Explorer directly to the directory containing all server log files.
* **Open Config**: Opens `config.ini` in Notepad for quick edits.
* **Exit**: Safely stops all active Node.js processes and closes the NodeRunner manager.

## Configuration (`config.ini`)

NodeRunner uses a simple `config.ini` file located alongside the executable. Multiple scripts can be configured using separate lines or comma-separated lists.

### Script Syntax Options
You can define each script entry using up to three pipe-delimited (`|`) segments:
`ScriptPath = Path/to/script.js | Optional Custom Name | Optional Custom Log Path`

```ini
[APP]
; REQUIRED: Specify script paths. Optionally append '| Server Name | CustomLogPath'
; If custom names or logs are omitted, descriptive defaults are automatically generated.

ScriptPath = D:/GBT/apk_server/index.js | APK Server | logs/apk_server.log
ScriptPath = D:/GBT/auth_service/app.js | Auth API
ScriptPath = D:/GBT/analytics/worker.js

; OPTIONAL: Custom Node binary path (Defaults to "node" if in system PATH)
NodePath = node

; Enable or disable background log writers (true/false)
EnableLogging = true

; Fallback master log file path
LogFile = logs/app.log

; Start NodeRunner automatically with Windows (true/false)
AutoStart = false

; Optional startup delay before launching configured services on application boot (in seconds)
Delay = 30

; Optional automatic retry count if a server fails to start or exits unexpectedly
RetryCount = 3

; Optional delay interval between retry attempts (in seconds)
RetryDelay = 5
```

## Building from Source

To compile NodeRunner from source, ensure Go is installed. The project embeds its system tray icon directly into the Windows executable using `rsrc`.

A `build.bat` script is provided to automatically fetch dependencies, embed resources, and compile the final windowless executable:

```batch
@echo off
go install github.com/akavel/rsrc@latest
go get golang.org/x/sys/windows/registry
go get github.com/getlantern/systray
go get gopkg.in/ini.v1
rsrc -ico icon.ico -o rsrc.syso
go build -ldflags="-H windowsgui" -o NodeRunner.exe
echo Built NodeRunner.exe successfully!
```

1. Ensure `icon.ico` is present in the project root directory.
2. Run `build.bat` in your terminal.
3. The resulting `NodeRunner.exe` will be ready for execution. The `-ldflags="-H windowsgui"` flag ensures the manager process itself runs completely stealthily without spawning a host console window.

# NodeRunner

NodeRunner is a lightweight, system-tray based Windows background service runner built in Go. It is specifically designed to quietly run a Node.js application in the background while providing an easy-to-use graphical interface from the system tray to manage the process.

## Features

* **System Tray Integration**: Manage your Node.js application directly from the Windows system tray.
* **Silent Execution**: Runs the Node.js process without a visible command prompt or console window.
* **Single Instance**: Ensures only one instance of NodeRunner operates at a time.
* **Auto-Start**: Can be configured to run automatically on Windows startup.
* **Easy Configuration**: Configured via a simple `config.ini` file.
* **Logging**: Outputs standard output and error to a configured log file.

## System Tray Menu

Right-clicking the NodeRunner system tray icon reveals the following options:

* **Restart Service**: Stops and restarts the background Node.js process.
* **Stop Service**: Terminates the background Node.js process.
* **Start Service**: Starts the Node.js process if it isn't currently running.
* **Run on Startup**: Toggles whether NodeRunner automatically starts with Windows.
* **Open Log**: Opens the configured log file in Notepad.
* **Open Config**: Opens `config.ini` in Notepad.
* **Exit**: Stops the Node.js process and closes NodeRunner.

## Configuration (`config.ini`)

NodeRunner uses a `config.ini` file for its settings. Ensure this file is in the same directory as the executable.

```ini
[APP]
# Path to the Node.js script you want to run
ScriptPath=path/to/your/script.js

# Path to the node executable (Defaults to "node" if in system PATH)
NodePath=node

# Enable or disable writing logs (true/false)
EnableLogging=true

# Path where logs will be saved (Defaults to "logs/app.log")
LogFile=logs/app.log

# Start NodeRunner automatically with Windows (true/false)
AutoStart=true
```

## Building from Source

To build NodeRunner from source, you need Go installed. The project relies on the `rsrc` tool to embed the system tray icon into the Windows executable.

A `build.bat` script is provided to automate the build process:

```batch
go install github.com/akavel/rsrc@latest
rsrc -ico icon.ico -o rsrc.syso
go build -ldflags="-H windowsgui" -o NodeRunner.exe
```

1. Ensure `icon.ico` is in the project directory.
2. Run `build.bat` in your terminal.
3. The output will be a `NodeRunner.exe` executable. Note the `-ldflags="-H windowsgui"` flag, which ensures the Go program itself runs without a console window.

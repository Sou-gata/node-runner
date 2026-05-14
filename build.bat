@echo off
go install github.com/akavel/rsrc@latest
go get golang.org/x/sys/windows/registry
go get github.com/getlantern/systray
go get gopkg.in/ini.v1
rsrc -ico icon.ico -o rsrc.syso
go build -ldflags="-H windowsgui" -o NodeRunner.exe
echo Built NodeRunner.exe successfully!

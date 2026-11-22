# Application build record

Go 1.27.4, Linux amd64 host; GOOS=windows, GOARCH=amd64, CGO_ENABLED=0.
Application build command: `bash build-cross-linux.sh` with the Go toolchain
on PATH. Uses `-buildvcs=false -trimpath -ldflags "-s -w -H windowsgui"`
for GUI programs. No external Go modules are required.

The host-native `dist/pmc-packager-host` was invoked with the Windows setup
stub and the freshly built FFmpeg directory. The result is a Windows PE
installer with a ZIP overlay, not a Linux executable. The matching media
build commands and configurations are in Perplex-FFmpeg-Source-1.1.6.zip.

See TEST-REPORT.md for checks performed and the Windows runtime limitation.

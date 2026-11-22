@echo off
setlocal
cd /d "%~dp0"
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

echo Running tests...
go test ./internal/...
if errorlevel 1 exit /b 1

echo Building converter (does not require FFmpeg or an older installation)...
go build -buildvcs=false -trimpath -ldflags "-s -w -H windowsgui" -o "converter.exe" ./cmd/converter
if errorlevel 1 exit /b 1
copy /Y "converter.exe" "cmd\setup\payload\Perplex Media Converter.exe" >nul
copy /Y "THIRD_PARTY.txt" "cmd\setup\payload\THIRD_PARTY.txt" >nul
copy /Y "README-WINDOWS.txt" "cmd\setup\payload\README-WINDOWS.txt" >nul
copy /Y "FFmpeg-REDISTRIBUTION.txt" "cmd\setup\payload\FFmpeg-REDISTRIBUTION.txt" >nul
copy /Y "LICENSE.txt" "cmd\setup\payload\LICENSE.txt" >nul
copy /Y "FFmpeg-GPLv3.txt" "cmd\setup\payload\FFmpeg-GPLv3.txt" >nul
copy /Y "FFMPEG-SOURCE.txt" "cmd\setup\payload\FFMPEG-SOURCE.txt" >nul
copy /Y "FFMPEG-MANIFEST.json" "cmd\setup\payload\FFMPEG-MANIFEST.json" >nul
if exist "cmd\setup\payload\licenses" rmdir /S /Q "cmd\setup\payload\licenses"
xcopy /E /I /Y "licenses" "cmd\setup\payload\licenses" >nul
if errorlevel 1 exit /b 1

echo Building offline setup stub...
go build -buildvcs=false -trimpath -ldflags "-s -w -H windowsgui" -o "setup-stub.exe" ./cmd/setup
if errorlevel 1 exit /b 1
copy /Y "setup-stub.exe" "cmd\offline-builder\payload\setup-stub.exe" >nul

echo Building graphical Offline Setup Builder...
go build -buildvcs=false -trimpath -ldflags "-s -w -H windowsgui" -o "Perplex-Media-Converter-Offline-Builder-1.1.6.exe" ./cmd/offline-builder
if errorlevel 1 exit /b 1

echo Building command-line installer packager...
go build -buildvcs=false -trimpath -o "pmc-packager.exe" ./cmd/packager
if errorlevel 1 exit /b 1

echo.
echo SUCCESS. The converter, setup stub, graphical builder, and packager were compiled.
echo No previous Perplex Media Converter installation is required.
echo To assemble a full offline setup, run build-offline-windows.cmd "C:\path\to\ffmpeg\bin".
endlocal

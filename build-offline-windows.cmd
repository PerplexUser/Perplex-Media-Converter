@echo off
setlocal
cd /d "%~dp0"
if not "%~1"=="" (
    set "FFMPEG_TOOLS=%~1"
) else if not "%FFMPEG_BIN%"=="" (
    set "FFMPEG_TOOLS=%FFMPEG_BIN%"
) else (
    set "FFMPEG_TOOLS=%CD%\third_party\ffmpeg\bin"
)

if not exist "%FFMPEG_TOOLS%\ffmpeg.exe" (
    echo ERROR: ffmpeg.exe was not found in: "%FFMPEG_TOOLS%"
    echo Use the verified tools from the 1.1.6 Offline Developer Kit.
    echo The matching Perplex-FFmpeg-Source-1.1.6.zip must accompany public downloads.
    exit /b 1
)
if not exist "%FFMPEG_TOOLS%\ffprobe.exe" (
    echo ERROR: ffprobe.exe was not found in: "%FFMPEG_TOOLS%"
    exit /b 1
)
set "PATH=%FFMPEG_TOOLS%;%PATH%"
call build-windows.cmd
if errorlevel 1 exit /b 1

if not exist dist mkdir dist
"pmc-packager.exe" -stub "setup-stub.exe" -ffmpeg-dir "%FFMPEG_TOOLS%" -out "dist\Perplex-Media-Converter-Setup-1.1.6-Offline.exe"
if errorlevel 1 exit /b 1

echo.
echo Ready: dist\Perplex-Media-Converter-Setup-1.1.6-Offline.exe
echo This installer requires NO download at installation time.
endlocal

# Perplex Media Converter 1.1.6 — Offline Edition

Windows 10/11 x64 media converter. English interface, local processing,
media previews, selectable audio tracks and optional external audio.
Original application: **MIT**, copyright (c) 2026 **Michael Lappenbusch**.
[Publisher website](https://www.perplex.click/).

## Downloads

The offline installer includes FFmpeg and FFprobe. No installation-time
or conversion-time downloads are needed. Download the installer from this
repository's Releases section or the publisher's website once published.

Every 1.1.6 binary download is accompanied by
**Perplex-FFmpeg-Source-1.1.6.zip**, the exact corresponding source package for
the bundled FFmpeg executables. It contains FFmpeg 9.1, x264, LAME 3.100,
MinGW-w64 sources, build instructions and configuration records.
GitHub's automatic "Source code" download does **not** replace this archive.

## What changed in 1.1.6

The unverified Gyan 1.1.5 binaries have been replaced with a dedicated FFmpeg
build produced from the included sources. Notices cover both codec libraries,
the compiler runtimes and the Independent JPEG Group. The installer copies
all notices. Its packagers refuse undocumented FFmpeg hashes.
See [FFMPEG-COMPLIANCE-STATUS.md](FFMPEG-COMPLIANCE-STATUS.md).

## Features

MKV → MP4; MP4 → MKV / MP3; AVI / MOV / WebM → MP4;
FLAC / WAV / M4A → MP3; MP3 → WAV. Audio track selection, external audio
replacement, H.264 quality/resolution controls, remuxing, progress,
cancellation, FFprobe metadata and 30-second previews.

The included build enables libx264, libmp3lame and FFmpeg's built-in codecs,
formats and filters. Hardware acceleration, network protocols and extra
Gyan-only libraries are not included. Those functions are not used by this
GUI. Files requiring an optional external decoder/filter absent from this
build may not work; the supported-format test matrix is in TEST-REPORT.md.

## Build the application from a clean checkout

Install Go 1.27.4 (the tested version) or a compatible Go toolchain.
On Windows, run `build-windows.cmd`. It runs tests and builds the converter,
setup stub, graphical offline builder and command-line packager.
No older Perplex installation or FFmpeg binary is needed for compilation.
The setup stub by itself is not an installable release asset.

## Build the complete offline installer

Use the **Offline Developer Kit 1.1.6** from the same release. It contains
the verified FFmpeg binaries and their matching source ZIP. Run:

```bat
build-offline-windows.cmd
```

From a source-only checkout, copy the exact `third_party/ffmpeg/bin` directory
from the 1.1.6 Developer Kit, or pass its location:

```bat
build-offline-windows.cmd "C:\path\to\1.1.6-tools\bin"
```

The packager enforces the binary-to-source mapping recorded in
`internal/offlinebundle/approved-release.json`. A deliberately changed FFmpeg
build needs updated sources and release metadata, not a renamed binary.
End users remain free to replace the installed executable files.

To rebuild FFmpeg itself, extract Perplex-FFmpeg-Source-1.1.6.zip and follow
its README.md. That build runs on Debian/Linux using a Windows cross compiler.
The included script builds from the local archived sources without fetching
media dependencies. Install the listed compiler tools beforehand.

## Publishing and licenses

Read [GITHUB-PUBLISHING.md](GITHUB-PUBLISHING.md) and
[RELEASE-CHECKLIST.md](RELEASE-CHECKLIST.md). Keep the matching source ZIP
publicly downloadable, free of charge, with a visible link beside every
installer/portable download. Include the provided notices and license texts.

The MIT application invokes FFmpeg via command-line processes and standard
media/JSON output. FFmpeg/FFprobe are separately licensed **GPLv3-or-later**.
Details: [THIRD_PARTY.txt](THIRD_PARTY.txt), [FFMPEG-SOURCE.txt](FFMPEG-SOURCE.txt)
and [FFMPEG-MANIFEST.json](FFMPEG-MANIFEST.json).
The full original upstream sources and notices are preserved.

The EXEs are unsigned. See TEST-REPORT.md for completed checks and the
remaining Windows GUI installation/uninstallation smoke test. Historical
1.1.5 licensing notes are retained only in `docs/history/`.

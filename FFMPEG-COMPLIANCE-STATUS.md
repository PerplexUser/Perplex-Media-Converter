# FFmpeg distribution preparation — 1.1.6

Prepared 22 November 2026. Scope: the files and hashes in this release only.

## Resolved issue

Version 1.1.5 shipped a large third-party Gyan build without an established
complete corresponding-source set. Adding a GPL text did not resolve it.
This version replaces those executables with a new FFmpeg 9.1 build from the
archived FFmpeg, x264 and LAME sources. The original Gyan binaries are not
included in any 1.1.6 release asset.

## Included evidence

- Exact input archives with SHA-256; FFmpeg's release signature was checked
  against its published signing-key fingerprint.
- Unmodified FFmpeg, x264 and LAME source trees in upstream archives.
- MinGW-w64 source and Debian packaging/patches, plus compiler package versions
  and hashes; compiler runtime licenses and GCC runtime exception.
- Build script, actual configuration and logs; exact executable/source mapping.
- Full applicable GPL/Library GPL texts, author credits and file notices,
  including IJG acknowledgement; Go runtime BSD notices for the application.
- GPLv3 FFmpeg license is distinguished from the MIT application.
- No nonfree flag, proprietary media dependency or automatic runtime download.
- Binary files can be replaced by recipients; no activation or vendor signing
  key is required to use compatible modified FFmpeg programs.

## What the publisher must do

Upload Perplex-FFmpeg-Source-1.1.6.zip beside every 1.1.6 installer or portable
binary and provide a conspicuous no-charge source link. The provided homepage
and GitHub release texts do this once all files are uploaded together.
Keep the matching sources available under GPLv3 section 6(d). Do not replace
the executables while keeping the old manifest/source package.
This preparation does not assert that any public download is already online.

## Scope and limits

This is a technical licensing remediation and source-delivery record, not a
legal opinion or a patent clearance. Process separation supports treating
the original command-line frontend as separately licensed; legal treatment
ultimately depends on the actual integration. Codec patent questions are
separate from copyright licensing. The source review does not establish
rights in user-supplied media or prove authorship of supplied artwork.
See TEST-REPORT.md for runtime testing limits.

Primary references, checked 22 November 2026:
- https://ffmpeg.org/legal.html
- https://www.gnu.org/licenses/gpl-3.0.html — especially sections 1, 5 and 6(d)
- https://www.gnu.org/licenses/gpl-faq.html — independent programs/aggregation
- https://ffmpeg.org/download.html — release signature/key
- https://www.videolan.org/developers/x264.html
- https://lame.sourceforge.io/license.txt
- https://gcc.gnu.org/onlinedocs/libstdc++/manual/license.html

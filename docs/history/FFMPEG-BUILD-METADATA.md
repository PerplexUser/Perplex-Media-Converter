# FFmpeg binary identification for Perplex Media Converter 1.1.5

These details were extracted from the bundled **Windows x64** executable
`ffmpeg.exe` and checked against `ffprobe.exe` hashes. They identify the
binary inputs used by the existing 1.1.5 offline installer and Developer Kit.
They do **not** constitute a complete corresponding-source distribution.

| Item | Identification |
|---|---|
| Build label (embedded) | `9.1-essentials_build-www.gyan.dev` |
| Build author/provider | Gyan Doshi (`https://www.gyan.dev/ffmpeg/builds/`) |
| FFmpeg license | GPLv3 (`--enable-gpl --enable-version3`) |
| `ffmpeg.exe` | 110,590,976 bytes; SHA-256 `ed8f66cb877087a96f5bbb40a2944f0e764f2ad8870d5cb647360d912e62bc07` |
| `ffprobe.exe` | 110,374,912 bytes; SHA-256 `df0199f606faa88e9ad192732a7c3987acfb647663534c297772e17730611a08` |
| Provider's public FFmpeg upstream source pointer for version 9.1 on 2026-11-22 | `https://github.com/FFmpeg/FFmpeg/commit/74e4393595` |

**Source precision caveat:** The provider's upstream commit URL is a starting
point, not independent proof that it, by itself, contains all of the precise
corresponding sources for the binaries above. The exact source revisions,
local patches, compiler/toolchain setup, build scripts and the sources for
**all included statically linked external libraries** remain to be verified
and bundled or supplied via equivalent access.

## Configuration string extracted from bundled ffmpeg.exe

```text
--enable-gpl --enable-version3 --enable-static --disable-w32threads --disable-autodetect --enable-cairo --enable-fontconfig --enable-iconv --enable-gnutls --enable-libxml2 --enable-gmp --enable-bzlib --enable-lzma --enable-zlib --enable-libcurl --enable-libsrt --enable-libssh --enable-libzmq --enable-avisynth --enable-sdl2 --enable-vapoursynth --enable-libwebp --enable-libx264 --enable-libx265 --enable-libxvid --enable-libaom --enable-libopenjpeg --enable-libvpx --enable-mediafoundation --enable-libass --enable-libfreetype --enable-libfribidi --enable-libharfbuzz --enable-libvidstab --enable-libvmaf --enable-libzimg --enable-amf --enable-cuda-llvm --enable-cuvid --enable-dxva2 --enable-d3d11va --enable-d3d12va --enable-ffnvcodec --enable-libvpl --enable-nvdec --enable-nvenc --enable-vaapi --enable-openal --enable-libgme --enable-libopenmpt --enable-libopencore-amrwb --enable-libmp3lame --enable-libtheora --enable-libvo-amrwbenc --enable-libgsm --enable-libopencore-amrnb --enable-libopus --enable-libspeex --enable-libvorbis --enable-librubberband
```

See `FFMPEG-COMPLIANCE-STATUS.md` and `RELEASE-CHECKLIST.md` before
publishing files that contain these binaries.

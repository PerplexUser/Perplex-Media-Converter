# Draft request for Gyan FFmpeg 9.1 build-source materials

This is a message you may send to the provider of the Windows binaries.
It has **not** been sent. Adjust details if your FFmpeg build changes.

**Subject:** Corresponding GPLv3 source and build information for Gyan FFmpeg 9.1 Essentials Windows x64

Hello,

I am preparing public distribution of a Windows application that includes
separate `ffmpeg.exe` and `ffprobe.exe` executables from your `9.1`
Essentials Windows x64 static build (`9.1-essentials_build-www.gyan.dev`).
The executables have these SHA-256 hashes:

- ffmpeg.exe: `ed8f66cb877087a96f5bbb40a2944f0e764f2ad8870d5cb647360d912e62bc07`
- ffprobe.exe: `df0199f606faa88e9ad192732a7c3987acfb647663534c297772e17730611a08`

Could you point me to (or provide) the complete corresponding source for
this **specific build**, including the FFmpeg source commit, patches, the
source versions for the statically linked third-party libraries and the
scripts/configuration/toolchain steps used to build these executables? I
would like to host the correct source package next to my binaries in a
GitHub Release in order to comply with GPLv3 section 6.

Thank you.

---

This request does not replace independently checking that the resulting
source materials are complete before redistribution.

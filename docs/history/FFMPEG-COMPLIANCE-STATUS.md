# FFmpeg licensing review — Perplex Media Converter 1.1.5

**Status on 22 November 2026: NOT YET READY for public redistribution of the
FFmpeg-containing binaries.** The assessment below documents requirements;
it is not a legal opinion or a certification of compliance.

## Verified facts from this project

- The Converter starts external `ffmpeg.exe` / `ffprobe.exe` processes. Its own
  original Go code and user interface are distributed under the **MIT License**
  with **Copyright (c) 2026 Michael Lappenbusch**.
- The binaries already included in the optional 1.1.5 Developer Kit and in
  the existing 1.1.5 Offline Setup are **Gyan FFmpeg 9.1 Essentials**, Windows
  x64, compiled with `--enable-gpl --enable-version3`, i.e. **GPLv3**.
- `FFmpeg-GPLv3.txt` contains the license text. `FFMPEG-BUILD-METADATA.md`
  records exact executable SHA-256 hashes and the embedded configure flags.
- The current packages **do not** include or provide confirmed equivalent
  access to the **complete corresponding source** for those specific compiled
  binaries and their statically linked dependencies. Simply linking to the
  upstream FFmpeg Git repository is not sufficient to establish this.

## What must be provided for GPLv3 binary distribution

For an internet download offered via GitHub Releases, GPLv3 Section 6(d)
permits distribution of object code if the **Corresponding Source** is
available from the **same designated download location**, by equivalent
access and at no extra charge. Include a clearly named corresponding-source
archive (or archives) and source instructions beside the setup and developer
kit assets. The distributed materials should include:

1. The exact FFmpeg source revision corresponding to the **binary actually
   shipped**, any downstream patches, source-level copyright notices and
   applicable GPLv3 license text.
2. Corresponding sources and license notices for third-party libraries that
   are included **inside these static binaries**, to the extent covered by
   the source-delivery obligations; identify exact versions and changes.
3. The build/configuration scripts and material required to build the relevant
   work, including external-library build recipes/patches and toolchain
   information where applicable. A configure line alone is not enough.
4. A release-level manifest that matches the source assets to **these exact
   binaries** (version, SHA-256 hashes, configure flags and dependency list).
5. In the app/installer notice and in the GitHub Release description, make
   FFmpeg attribution and its **separate GPLv3 license** explicit. Do not
   describe FFmpeg as MIT-licensed.

Do not rely on a volatile "latest" URL or on the user independently obtaining
sources from a third-party site after redistribution. Preserve stable source
availability yourself for as long as the corresponding binaries remain offered.

## Is the own MIT-licensed converter affected?

Because this Go application launches FFmpeg as a **separate command-line
process**, rather than directly linking GPL FFmpeg libraries into its own
binary, separate licensing for the original application is supportable.
Whether programs are legally independent or a derivative combined work depends
on the specifics of their integration; this is **not** a blanket guarantee.
The **installer/package** must still meet FFmpeg's source and notice duties.

## Current blocker / how to finish

The exact **full source and build inputs of Gyan FFmpeg 9.1 Essentials**, and
sources/build data for all relevant statically linked libraries, have not
been assembled or independently matched to these binaries. Confirm these
materials with the build provider or use a self-produced FFmpeg build with a
fully archived, reproducible source/build environment. Once verified, attach
that material to the same GitHub release as the Developer Kit and Setup.

**Until then, publish the source-only GitHub repository, but do not publish
or redistribute the FFmpeg-containing Developer Kit or Offline Installer as
public release assets.** The existing local files are useful for private
build/testing, not evidence of completed compliance.

## Primary sources

- FFmpeg licensing: https://ffmpeg.org/legal.html
- Gyan FFmpeg builds & declared GPLv3 licensing: https://www.gyan.dev/ffmpeg/builds/
- GNU GPLv3 full terms, particularly section 6: https://www.gnu.org/licenses/gpl-3.0.html
- GNU FAQ on independent programs / aggregates: https://www.gnu.org/licenses/gpl-faq.en.html
- FFmpeg 9.1 source pointer shown by Gyan: https://github.com/FFmpeg/FFmpeg/commit/74e4393595

This document provides practical compliance guidance, **not legal advice**.

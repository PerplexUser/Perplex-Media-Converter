#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
export CGO_ENABLED=0
# A complete source-only build; FFmpeg is needed only by the final packager.
go test -count=1 ./internal/...
mkdir -p dist cmd/setup/payload/licenses
GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags '-s -w -H windowsgui' -o converter.exe ./cmd/converter
cp converter.exe 'cmd/setup/payload/Perplex Media Converter.exe'
for name in LICENSE.txt THIRD_PARTY.txt README-WINDOWS.txt FFmpeg-GPLv3.txt FFmpeg-REDISTRIBUTION.txt FFMPEG-SOURCE.txt FFMPEG-MANIFEST.json; do
    cp "$name" "cmd/setup/payload/$name"
done
cp -R licenses/. cmd/setup/payload/licenses/
GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags '-s -w -H windowsgui' -o setup-stub.exe ./cmd/setup
cp setup-stub.exe cmd/offline-builder/payload/setup-stub.exe
GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags '-s -w -H windowsgui' -o Perplex-Media-Converter-Offline-Builder-1.1.6.exe ./cmd/offline-builder
GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags '-s -w' -o pmc-packager.exe ./cmd/packager
# Host-native packager can append the ZIP payload to the Windows setup stub.
go build -buildvcs=false -trimpath -o dist/pmc-packager-host ./cmd/packager
printf 'Built all Windows executables and the host packager.\n'

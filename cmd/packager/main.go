// Perplex Media Converter offline installer packager.
// Usage: pmc-packager -stub setup-stub.exe -ffmpeg-dir C:\ffmpeg\bin -out dist\Perplex-Media-Converter-Setup-1.1.6-Offline.exe
// Compiles on any Go-supported OS, but its inputs must be Windows executables.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"perplex-media-converter/internal/offlinebundle"
)

func main() {
	stubPath := flag.String("stub", "setup-stub.exe", "compiled Windows setup stub")
	toolsDir := flag.String("ffmpeg-dir", filepath.Join("third_party", "ffmpeg", "bin"), "directory containing ffmpeg.exe and ffprobe.exe")
	ffmpeg := flag.String("ffmpeg", "", "override ffmpeg.exe location")
	ffprobe := flag.String("ffprobe", "", "override ffprobe.exe location")
	out := flag.String("out", filepath.Join("dist", "Perplex-Media-Converter-Setup-1.1.6-Offline.exe"), "output installer")
	flag.Parse()
	if *ffmpeg == "" {
		*ffmpeg = filepath.Join(*toolsDir, "ffmpeg.exe")
	}
	if *ffprobe == "" {
		*ffprobe = filepath.Join(*toolsDir, "ffprobe.exe")
	}
	stub, err := os.ReadFile(*stubPath)
	if err != nil {
		fail("read setup stub", err)
	}
	fmt.Println("FFmpeg: ", *ffmpeg)
	fmt.Println("FFprobe:", *ffprobe)
	fmt.Println("Output: ", *out)
	fmt.Println("The installer will be offline. FFmpeg licensing is separate from the MIT application license.")
	err = offlinebundle.Build(context.Background(), *out, stub, *ffmpeg, *ffprobe, func(label string, percent int) {
		fmt.Printf("%3d%%  %s\n", percent, label)
	})
	if err != nil {
		fail("create offline installer", err)
	}
	pkg, err := offlinebundle.Open(*out)
	if err != nil {
		fail("verify offline installer", err)
	}
	if err = pkg.Close(); err != nil {
		fail("close verified installer", err)
	}
	fmt.Println("SUCCESS:", *out)
}
func fail(what string, err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", what+":", err)
	os.Exit(1)
}

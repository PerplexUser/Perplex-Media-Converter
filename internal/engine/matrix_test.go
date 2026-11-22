package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A functional test of all ten web-app conversion paths, not only command syntax.
// Media fixtures are short, but each output is independently checked by FFprobe.
func TestAllTenConversions(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	for input, targets := range Conversions {
		for _, target := range targets {
			t.Run(strings.ToUpper(input)+"_to_"+strings.ToUpper(target), func(t *testing.T) {
				srcPath := fixturePath(t, "source."+input)
				info, err := Probe(ffprobe, srcPath)
				if err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(dir, input+"_result."+target)
				opts := Defaults(srcPath, target)
				opts.Output = output
				args, err := BuildArgs(opts, info, nil)
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(ffmpeg, args...)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("FFmpeg %s -> %s: %v; %s", input, target, err, out)
				}
				if fi, err := os.Stat(output); err != nil || fi.Size() == 0 {
					t.Fatalf("empty output: %v", err)
				}
				parsed, err := Probe(ffprobe, output)
				if err != nil {
					t.Fatal(err)
				}
				kind := "video"
				if target == "mp3" || target == "wav" {
					kind = "audio"
				}
				found := false
				for _, s := range parsed.Streams {
					if s.CodecType == kind {
						found = true
					}
				}
				if !found {
					t.Fatalf("output lacks %s: %v", kind, parsed.Streams)
				}
			})
		}
	}
}

func TestCompatiblePreview(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	input := fixturePath(t, "source.mkv")
	output := filepath.Join(dir, "preview.mp4")
	info, err := Probe(ffprobe, input)
	if err != nil {
		t.Fatal(err)
	}
	opts := Defaults(input, "mp4")
	args, err := PreviewArgs(opts, info, nil, output)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("preview: %s (%v)", out, err)
	}
	result, err := Probe(ffprobe, output)
	if err != nil {
		t.Fatal(err)
	}
	video, hasVideo := result.FirstVideo()
	if !hasVideo || video.CodecName != "h264" {
		t.Fatal("preview not H.264", video)
	}
	if len(result.AudioTracks()) != 1 || result.AudioTracks()[0].CodecName != "aac" {
		t.Fatal("preview not AAC", result.AudioTracks())
	}
}

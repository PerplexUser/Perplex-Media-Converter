package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fakeInfo() MediaInfo {
	var m MediaInfo
	m.Streams = []Stream{{Index: 0, CodecType: "video", CodecName: "h264"}, {Index: 1, CodecType: "audio", CodecName: "aac", Channels: 2, Tags: map[string]string{"language": "deu"}}, {Index: 2, CodecType: "audio", CodecName: "ac3", Channels: 6, Tags: map[string]string{"language": "eng", "title": "Original"}}}
	m.Format.Duration = "22.5"
	return m
}
func TestTargets(t *testing.T) {
	cases := map[string]string{"a.mkv": "mp4", "a.mp4": "mkv,mp3", "a.avi": "mp4", "a.mov": "mp4", "a.webm": "mp4", "a.flac": "mp3", "a.wav": "mp3", "a.mp3": "wav", "a.m4a": "mp3"}
	for in, out := range cases {
		if got := strings.Join(Targets(in), ","); got != out {
			t.Errorf("%s: %s != %s", in, got, out)
		}
	}
}
func TestSelectedTracks(t *testing.T) {
	opts := Defaults("movie.mkv", "mp4")
	opts.Output = "out.mp4"
	opts.EmbeddedTrack = 2
	args, err := BuildArgs(opts, fakeInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "-map 0:2") {
		t.Fatal(args)
	}
	opts.AudioMode = AudioExternal
	opts.ExternalFile = "music.flac"
	opts.ExternalTrack = 0
	ext := MediaInfo{Streams: []Stream{{Index: 0, CodecType: "audio", CodecName: "flac"}}}
	args, err = BuildArgs(opts, fakeInfo(), &ext)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-map 1:0") || strings.Contains(joined, "-map 0:1") || strings.Contains(joined, "-map 0:2") {
		t.Fatal(args)
	}
	opts.AudioMode = AudioNone
	args, err = BuildArgs(opts, fakeInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "-an") {
		t.Fatal(args)
	}
}
func TestCopyAndAudio(t *testing.T) {
	opts := Defaults("movie.mp4", "mkv")
	opts.Output = "out.mkv"
	args, err := BuildArgs(opts, fakeInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "-c:v copy -c:a copy") {
		t.Fatal(args)
	}
	opts = Defaults("movie.mp4", "mp3")
	opts.Output = "out.mp3"
	opts.EmbeddedTrack = 2
	args, err = BuildArgs(opts, fakeInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "-map 0:2 -vn") {
		t.Fatal(args)
	}
}
func TestMissingTrack(t *testing.T) {
	opts := Defaults("movie.mkv", "mp4")
	opts.Output = "out.mp4"
	opts.EmbeddedTrack = 99
	if _, err := BuildArgs(opts, fakeInfo(), nil); err == nil {
		t.Fatal("Expected error")
	}
}
func TestRealFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	video := fixturePath(t, "multi.mkv")
	sound := fixturePath(t, "external.wav")
	output := filepath.Join(dir, "result.mp4")
	info, err := Probe(ffprobe, video)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.AudioTracks()) != 2 {
		t.Fatalf("expected two soundtracks, got %v", info.AudioTracks())
	}
	ext, err := Probe(ffprobe, sound)
	if err != nil {
		t.Fatal(err)
	}
	opts := Defaults(video, "mp4")
	opts.Output = output
	opts.AudioMode = AudioExternal
	opts.ExternalFile = sound
	opts.ExternalTrack = ext.AudioTracks()[0].Index
	args, err := BuildArgs(opts, info, &ext)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("convert: %s %v", out, err)
	}
	if stat, err := os.Stat(output); err != nil || stat.Size() == 0 {
		t.Fatalf("empty output: %v", err)
	}
	result, err := Probe(ffprobe, output)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AudioTracks()) != 1 {
		t.Fatalf("expected one soundtrack, got %v", result.AudioTracks())
	}
}

// Stored fixtures let tests exercise the shipped tools without requiring extra
// encoders or the disabled lavfi input device just to create test inputs.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return path
}

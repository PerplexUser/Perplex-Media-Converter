// Package engine implements local FFmpeg-based conversion rules used by the Windows GUI.
// It intentionally has no platform-specific dependencies and never invokes a shell.
package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var Conversions = map[string][]string{
	"mkv": {"mp4"}, "mp4": {"mkv", "mp3"}, "avi": {"mp4"}, "mov": {"mp4"},
	"webm": {"mp4"}, "flac": {"mp3"}, "wav": {"mp3"}, "mp3": {"wav"}, "m4a": {"mp3"},
}
var AudioExtensions = []string{".aac", ".ac3", ".aif", ".aiff", ".alac", ".flac", ".m4a", ".mka", ".mp3", ".ogg", ".opus", ".wav", ".wma", ".mp4", ".mkv"}

type MediaInfo struct {
	Streams []Stream `json:"streams"`
	Format  struct {
		Filename   string `json:"filename"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		FormatName string `json:"format_name"`
	} `json:"format"`
}
type Stream struct {
	Index         int               `json:"index"`
	CodecType     string            `json:"codec_type"`
	CodecName     string            `json:"codec_name"`
	SampleRate    string            `json:"sample_rate"`
	Channels      int               `json:"channels"`
	ChannelLayout string            `json:"channel_layout"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Duration      string            `json:"duration"`
	Tags          map[string]string `json:"tags"`
	Disposition   struct {
		AttachedPic int `json:"attached_pic"`
		Default     int `json:"default"`
	} `json:"disposition"`
}

func (m MediaInfo) AudioTracks() []Stream {
	out := make([]Stream, 0)
	for _, s := range m.Streams {
		if s.CodecType == "audio" {
			out = append(out, s)
		}
	}
	return out
}
func (m MediaInfo) FirstVideo() (Stream, bool) {
	for _, s := range m.Streams {
		if s.CodecType == "video" && s.Disposition.AttachedPic == 0 {
			return s, true
		}
	}
	return Stream{}, false
}
func (m MediaInfo) DurationSeconds() float64 {
	n, _ := strconv.ParseFloat(m.Format.Duration, 64)
	if n > 0 {
		return n
	}
	for _, s := range m.Streams {
		n, _ = strconv.ParseFloat(s.Duration, 64)
		if n > 0 {
			return n
		}
	}
	return 0
}
func (s Stream) Label() string {
	lang := strings.TrimSpace(s.Tags["language"])
	if lang == "" || lang == "und" {
		lang = "Unknown"
	}
	codec := strings.ToUpper(s.CodecName)
	if codec == "" {
		codec = "?"
	}
	desc := fmt.Sprintf("Track %d  ·  %s  ·  %s", s.Index, lang, codec)
	if s.Channels > 0 {
		desc += fmt.Sprintf("  ·  %d channels", s.Channels)
	}
	if title := strings.TrimSpace(s.Tags["title"]); title != "" {
		desc += " · " + title
	}
	if s.Disposition.Default == 1 {
		desc += "  ·  Default"
	}
	return desc
}
func ParseProbe(data []byte) (MediaInfo, error) {
	var m MediaInfo
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("Invalid FFprobe response: %w", err)
	}
	if len(m.Streams) == 0 {
		return m, errors.New("This file has no recognizable audio or video streams.")
	}
	return m, nil
}
func Probe(probeExe, path string) (MediaInfo, error) {
	cmd := exec.Command(probeExe, "-v", "error", "-show_format", "-show_streams", "-of", "json", path)
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return MediaInfo{}, fmt.Errorf("FFprobe could not analyze the file: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return ParseProbe(out)
}
func Extension(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}
func Targets(path string) []string { return Conversions[Extension(path)] }
func Supports(path, target string) bool {
	for _, x := range Targets(path) {
		if x == target {
			return true
		}
	}
	return false
}

// AudioMode specifies precisely one selected audio stream, a separate audio stream,
// or no audio. A separate stream REPLACES the embedded one; it never mixes them.
type AudioMode string

const (
	AudioEmbedded AudioMode = "embedded"
	AudioExternal AudioMode = "external"
	AudioNone     AudioMode = "none"
)

type Options struct {
	Input             string
	Output            string
	Target            string
	Mode              string // "encode" or "copy" (video only)
	Quality           string // x264 CRF 18, 23, 28
	Resolution        string // original,1080,720,480
	Preset            string // ultrafast,veryfast,medium
	VideoAudioBitrate int    // 128..320
	AudioBitrate      int
	SampleRate        string // original, 44100,48000
	Channels          int    // 1 or 2
	AudioMode         AudioMode
	EmbeddedTrack     int // Absolute stream index, not order of audio streams
	ExternalFile      string
	ExternalTrack     int // Absolute stream index in external file
}

func Defaults(input, target string) Options {
	mode := "encode"
	if Extension(input) == "mp4" && target == "mkv" {
		mode = "copy"
	}
	return Options{Input: input, Target: target, Mode: mode, Quality: "23", Resolution: "original", Preset: "veryfast", VideoAudioBitrate: 192, AudioBitrate: 192, SampleRate: "original", Channels: 2, AudioMode: AudioEmbedded, EmbeddedTrack: -1, ExternalTrack: -1}
}
func contains(s string, x ...string) bool {
	for _, a := range x {
		if s == a {
			return true
		}
	}
	return false
}
func validBitrate(n int) bool { return n == 128 || n == 192 || n == 256 || n == 320 }
func selectedAudio(opts Options, src MediaInfo, external *MediaInfo) (fileNum, streamIndex int, found bool, selected Stream) {
	if opts.AudioMode == AudioNone {
		return 0, 0, false, Stream{}
	}
	info := src
	idx := opts.EmbeddedTrack
	if opts.AudioMode == AudioExternal {
		if external == nil {
			return 0, 0, false, Stream{}
		}
		info = *external
		idx = opts.ExternalTrack
		fileNum = 1
	}
	tracks := info.AudioTracks()
	if len(tracks) == 0 {
		return 0, 0, false, Stream{}
	}
	if idx < 0 {
		return fileNum, tracks[0].Index, true, tracks[0]
	}
	for _, track := range tracks {
		if track.Index == idx {
			return fileNum, idx, true, track
		}
	}
	return 0, 0, false, Stream{}
}
func BuildArgs(opts Options, src MediaInfo, external *MediaInfo) ([]string, error) {
	if !Supports(opts.Input, opts.Target) {
		return nil, fmt.Errorf("Unsupported conversion %s → %s", Extension(opts.Input), opts.Target)
	}
	if opts.Input == "" || opts.Output == "" {
		return nil, errors.New("Source and destination paths are required.")
	}
	abs1, _ := filepath.Abs(opts.Input)
	abs2, _ := filepath.Abs(opts.Output)
	if strings.EqualFold(abs1, abs2) {
		return nil, errors.New("Source and destination files must be different.")
	}
	if !contains(opts.Mode, "copy", "encode") || !contains(opts.Quality, "18", "23", "28") || !contains(opts.Resolution, "original", "1080", "720", "480") || !contains(opts.Preset, "ultrafast", "veryfast", "medium") || !contains(opts.SampleRate, "original", "44100", "48000") || !validBitrate(opts.AudioBitrate) || !validBitrate(opts.VideoAudioBitrate) || !contains(strconv.Itoa(opts.Channels), "1", "2") {
		return nil, errors.New("One or more conversion settings are invalid.")
	}
	if !contains(string(opts.AudioMode), string(AudioEmbedded), string(AudioExternal), string(AudioNone)) {
		return nil, errors.New("Invalid audio track selection.")
	}
	if opts.AudioMode == AudioExternal && (opts.ExternalFile == "" || external == nil) {
		return nil, errors.New("Please choose and analyze a separate audio file.")
	}
	args := []string{"-hide_banner", "-nostdin", "-y", "-i", opts.Input}
	if opts.AudioMode == AudioExternal {
		args = append(args, "-i", opts.ExternalFile)
	}
	args = append(args, "-map_metadata", "0")
	num, idx, hasAudio, track := selectedAudio(opts, src, external)
	if opts.AudioMode != AudioNone && !hasAudio {
		return nil, errors.New("The selected audio track does not exist.")
	}
	isVideo := opts.Target == "mp4" || opts.Target == "mkv"
	if isVideo {
		v, ok := src.FirstVideo()
		if !ok {
			return nil, errors.New("The source does not contain a usable video track.")
		}
		args = append(args, "-map", fmt.Sprintf("0:%d", v.Index))
		if hasAudio {
			args = append(args, "-map", fmt.Sprintf("%d:%d", num, idx))
		}
		args = append(args, "-sn", "-dn")
		if opts.Mode == "copy" {
			args = append(args, "-c:v", "copy")
			if hasAudio {
				args = append(args, "-c:a", "copy")
			}
		} else {
			scale := "scale=trunc(iw/2)*2:trunc(ih/2)*2"
			limits := map[string][2]int{"1080": {1920, 1080}, "720": {1280, 720}, "480": {854, 480}}
			if size, ok := limits[opts.Resolution]; ok {
				scale = fmt.Sprintf("scale=w='min(iw,%d)':h='min(ih,%d)':force_original_aspect_ratio=decrease:force_divisible_by=2", size[0], size[1])
			}
			args = append(args, "-c:v", "libx264", "-preset", opts.Preset, "-crf", opts.Quality, "-vf", scale, "-pix_fmt", "yuv420p")
			if hasAudio {
				args = append(args, "-c:a", "aac", "-b:a", fmt.Sprintf("%dk", opts.VideoAudioBitrate))
			}
		}
		if !hasAudio {
			args = append(args, "-an")
		}
		// External audio must not extend a movie beyond the original video's duration.
		if opts.AudioMode == AudioExternal && src.DurationSeconds() > 0 {
			args = append(args, "-t", strconv.FormatFloat(src.DurationSeconds(), 'f', 3, 64))
		}
		if opts.Target == "mp4" {
			args = append(args, "-movflags", "+faststart")
		}
	} else {
		if !hasAudio {
			return nil, errors.New("MP3 and WAV exports require an audio track.")
		}
		args = append(args, "-map", fmt.Sprintf("%d:%d", num, idx), "-vn", "-sn", "-dn", "-ac", strconv.Itoa(opts.Channels))
		if opts.Target == "mp3" {
			args = append(args, "-c:a", "libmp3lame", "-b:a", fmt.Sprintf("%dk", opts.AudioBitrate), "-id3v2_version", "3")
			if opts.SampleRate == "original" {
				// LAME does not support arbitrary input rates.
				supported := []int{8000, 11025, 12000, 16000, 22050, 24000, 32000, 44100, 48000}
				sourceRate, _ := strconv.Atoi(track.SampleRate)
				if sourceRate == 0 {
					sourceRate = 44100
				}
				best := supported[0]
				dist := abs(sourceRate - best)
				for _, s := range supported[1:] {
					if d := abs(sourceRate - s); d < dist {
						dist = d
						best = s
					}
				}
				args = append(args, "-ar", strconv.Itoa(best))
			}
		} else {
			args = append(args, "-c:a", "pcm_s16le", "-rf64", "auto")
		}
		if opts.SampleRate != "original" {
			args = append(args, "-ar", opts.SampleRate)
		}
	}
	args = append(args, "-progress", "pipe:1", "-nostats", opts.Output)
	return args, nil
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// PreviewArgs creates an MP4 excerpt (max. 30 seconds) for codecs unsupported
// by native Windows playback. It honors the user's selected or external track.
func PreviewArgs(opts Options, src MediaInfo, external *MediaInfo, output string) ([]string, error) {
	v, ok := src.FirstVideo()
	if !ok {
		return nil, errors.New("No video stream available for preview.")
	}
	args := []string{"-hide_banner", "-nostdin", "-y", "-i", opts.Input}
	if opts.AudioMode == AudioExternal {
		if opts.ExternalFile == "" || external == nil {
			return nil, errors.New("Please select a separate audio file.")
		}
		args = append(args, "-i", opts.ExternalFile)
	}
	args = append(args, "-t", "30", "-map", fmt.Sprintf("0:%d", v.Index))
	num, idx, hasAudio, _ := selectedAudio(opts, src, external)
	if opts.AudioMode != AudioNone && !hasAudio {
		return nil, errors.New("The selected audio track does not exist.")
	}
	if hasAudio {
		args = append(args, "-map", fmt.Sprintf("%d:%d", num, idx))
	}
	args = append(args, "-sn", "-dn", "-map_metadata", "-1", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "24",
		"-vf", "scale=w='min(iw,960)':h='min(ih,540)':force_original_aspect_ratio=decrease:force_divisible_by=2", "-pix_fmt", "yuv420p")
	if hasAudio {
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "44100")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-movflags", "+faststart", "-progress", "pipe:1", "-nostats", output)
	return args, nil
}
func CheckPaths(opts Options) error {
	if _, err := os.Stat(opts.Input); err != nil {
		return fmt.Errorf("Cannot read source file: %w", err)
	}
	if opts.AudioMode == AudioExternal {
		if _, err := os.Stat(opts.ExternalFile); err != nil {
			return fmt.Errorf("Cannot read separate audio file: %w", err)
		}
	}
	dir := filepath.Dir(opts.Output)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("Destination folder does not exist: %s", dir)
	}
	return nil
}

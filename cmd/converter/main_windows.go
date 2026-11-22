//go:build windows

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"perplex-media-converter/internal/engine"
	"perplex-media-converter/internal/winui"
)

const (
	appTitle     = "Perplex Media Converter"
	eventMessage = winui.WM_APP + 12
)

type appEvent struct {
	kind    string
	path    string
	info    engine.MediaInfo
	err     error
	gen     int
	text    string
	percent int
	output  string
}
type App struct {
	main                           winui.HWND
	widgets                        map[string]winui.HWND
	ids                            map[int]string
	font, headingFont, captionFont uintptr
	logo                           uintptr
	nextID                         int
	currentInput                   string
	currentExternal                string
	info                           *engine.MediaInfo
	externalInfo                   *engine.MediaInfo
	targetList                     []string
	trackIndices                   []int
	genInput, genExternal          int
	ready                          bool
	busy                           bool
	cancel                         context.CancelFunc
	started                        time.Time
	lastResult                     string
	ffmpeg, ffprobe                string
	events                         chan appEvent
	previewFiles                   []string
}

var application *App

func proc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	a := application
	if a != nil && a.main != 0 && winui.HWND(hwnd) == a.main {
		switch msg {
		case winui.WM_ERASEBKGND:
			return 1
		case winui.WM_PAINT:
			a.draw()
			return 0
		case winui.WM_CTLCOLORSTATIC:
			return a.paintControl(wp, lp, false)
		case winui.WM_CTLCOLOREDIT:
			return a.paintControl(wp, lp, true)
		case winui.WM_DRAWITEM:
			a.drawButton(int(wp), lp)
			return 1
		case winui.WM_COMMAND:
			a.command(int(wp&0xffff), int((wp>>16)&0xffff))
			return 0
		case winui.WM_SIZE:
			a.layout()
			return 0
		case winui.WM_DROPFILES:
			if !a.busy {
				a.chooseInput(winui.DraggedPath(wp))
			} else {
				_ = winui.DraggedPath(wp)
			}
			return 0
		case eventMessage:
			a.handleEvents()
			return 0
		case winui.WM_TIMER:
			if a.busy {
				winui.SetText(a.widgets["elapsed"], "Elapsed: "+durationDisplay(time.Since(a.started)))
			}
			return 0
		case winui.WM_CLOSE:
			if a.busy && winui.Msg(a.main, "Cancel current job?", "A conversion is running. Cancel it and close the application?", winui.MB_YESNO|winui.MB_ICONQUESTION) != winui.IDYES {
				return 0
			}
			if a.cancel != nil {
				a.cancel()
			}
			winui.Destroy(a.main)
			return 0
		case winui.WM_DESTROY:
			if a.cancel != nil {
				a.cancel()
			}
			a.cleanupPreview()
			winui.FreeIcon(a.logo)
			winui.PostQuit(0)
			return 0
		}
	}
	return winui.DefWndProc(winui.HWND(hwnd), msg, wp, lp)
}
func main() {
	runtime.LockOSThread()
	a := &App{widgets: map[string]winui.HWND{}, ids: map[int]string{}, events: make(chan appEvent, 1024)}
	application = a
	a.ffmpeg, a.ffprobe = locateTools()
	a.ready = a.ffmpeg != "" && a.ffprobe != ""
	winui.CommonControls()
	err := winui.Register("PerplexMediaConverterV2", syscall.NewCallback(proc))
	if err != nil {
		winui.Msg(0, appTitle, err.Error(), winui.MB_ICONERROR)
		return
	}
	a.main, err = winui.CreateMain("PerplexMediaConverterV2", appTitle, 1120, 826)
	if err != nil {
		winui.Msg(0, appTitle, err.Error(), winui.MB_ICONERROR)
		return
	}
	a.font = winui.Font(16, false)
	a.headingFont = winui.Font(29, true)
	a.captionFont = winui.Font(14, false)
	a.makeUI()
	a.logo = winui.LoadIcon(winui.MainIconPath())
	winui.SetIconFromHandle(a.main, a.logo)
	winui.DragAccept(a.main)
	a.layout()
	a.updateEnabled()
	if !a.ready {
		a.setStatus("FFmpeg not found. Please run the installer again or install FFmpeg.")
		winui.Msg(a.main, "FFmpeg is missing", "Conversion requires ffmpeg.exe and ffprobe.exe. Please run the full Offline Setup again, or install FFmpeg and restart the application.", winui.MB_OK|winui.MB_ICONWARNING)
	} else {
		a.setStatus("Ready  ·  Files stay on your PC. Nothing is uploaded.")
	}
	winui.Show(a.main)
	winui.Loop()
}
func locateTools() (string, string) {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	candidates := []string{dir, filepath.Join(dir, "tools"), filepath.Join(dir, "bin"), `C:\ffmpeg\bin`}
	for _, base := range candidates {
		f, p := filepath.Join(base, "ffmpeg.exe"), filepath.Join(base, "ffprobe.exe")
		if stat, err := os.Stat(f); err == nil && !stat.IsDir() {
			if stat2, err := os.Stat(p); err == nil && !stat2.IsDir() {
				return f, p
			}
		}
	}
	f, e1 := exec.LookPath("ffmpeg.exe")
	p, e2 := exec.LookPath("ffprobe.exe")
	if e1 == nil && e2 == nil {
		return f, p
	}
	return "", ""
}
func (a *App) emit(ev appEvent) {
	select {
	case a.events <- ev:
		winui.Post(a.main, eventMessage, 0, 0)
	default:
		// Progress and verbose stderr can be dropped under load; completion and errors cannot.
		if ev.kind == "done" || strings.HasPrefix(ev.kind, "probe") {
			go func() { a.events <- ev; winui.Post(a.main, eventMessage, 0, 0) }()
		}
	}
}
func (a *App) label(name, text string, bold bool) {
	a.add(name, "STATIC", text, winui.SS_LEFT, 0, bold)
}
func (a *App) add(name, class, text string, style, extra uint32, bold bool) {
	a.nextID++
	id := 1000 + a.nextID
	if class == "BUTTON" {
		style = (style &^ winui.BS_DEFPUSHBUTTON) | winui.BS_OWNERDRAW
	}
	hwnd := winui.CreateControl(a.main, class, text, style, extra, id)
	a.widgets[name] = hwnd
	a.ids[id] = name
	font := a.font
	if name == "title" {
		font = a.headingFont
	} else if name == "subtitle" || name == "heroBadge" || name == "status" || name == "fileInfo" || name == "foot" || name == "elapsed" {
		font = a.captionFont
	}
	if bold && name != "title" {
		font = a.font
	}
	winui.SetFont(hwnd, font)
}
func (a *App) combo(name string, values []string, sel int) {
	a.add(name, "COMBOBOX", "", winui.WS_TABSTOP|winui.CBS_DROPDOWNLIST, 0, false)
	for _, v := range values {
		winui.AddCombo(a.widgets[name], v)
	}
	if len(values) > 0 {
		winui.SelectCombo(a.widgets[name], sel)
	}
}
func (a *App) makeUI() {
	a.label("title", appTitle, true)
	a.label("subtitle", "VIDEO  /  AUDIO  /  YOUR WORKFLOW", false)
	a.label("heroBadge", "100% LOCAL", true)
	a.label("inputLabel", "01   SOURCE FILE", true)
	a.add("input", "EDIT", "", winui.WS_BORDER|winui.ES_AUTOHSCROLL|winui.ES_READONLY, winui.WS_EX_CLIENTEDGE, false)
	a.add("inputBrowse", "BUTTON", "Browse files", winui.WS_TABSTOP, 0, false)
	a.label("fileInfo", "Drop a file here, or browse  ·  MKV, MP4, AVI, MOV, WebM, FLAC, WAV, MP3, M4A", false)
	a.label("audioModeLabel", "02   AUDIO SOURCE", true)
	a.label("trackLabel", "TRACK SELECTION  /  LANGUAGE  /  CODEC", true)
	a.combo("audioMode", []string{"Use embedded audio track", "Use separate audio file", "No audio (video only)"}, 0)
	a.combo("track", []string{"Select a source file first"}, 0)
	a.label("externalLabel", "EXTERNAL AUDIO FILE  ·  REPLACES THE ORIGINAL SOUND", true)
	a.add("external", "EDIT", "", winui.WS_BORDER|winui.ES_AUTOHSCROLL|winui.ES_READONLY, winui.WS_EX_CLIENTEDGE, false)
	a.add("externalBrowse", "BUTTON", "Browse audio", winui.WS_TABSTOP, 0, false)
	a.label("formatLabel", "03   OUTPUT FORMAT", true)
	a.label("outputLabel", "DESTINATION FILE", true)
	a.combo("format", []string{"Select a source first"}, 0)
	a.add("output", "EDIT", "", winui.WS_BORDER|winui.ES_AUTOHSCROLL, winui.WS_EX_CLIENTEDGE, false)
	a.add("outputBrowse", "BUTTON", "Save as", winui.WS_TABSTOP, 0, false)
	a.label("settingsLabel", "VIDEO SETTINGS", true)
	a.label("modeLabel", "ENCODING MODE", true)
	a.label("qualityLabel", "VIDEO QUALITY", true)
	a.label("resLabel", "MAXIMUM RESOLUTION", true)
	a.combo("mode", []string{"Re-encode  ·  H.264 + AAC", "Fast  ·  Copy compatible streams"}, 0)
	a.combo("quality", []string{"High quality (CRF 18)", "Balanced (CRF 23)", "Smaller file (CRF 28)"}, 1)
	a.combo("resolution", []string{"Original (never upscale)", "Up to 1080p", "Up to 720p", "Up to 480p"}, 0)
	a.label("presetLabel", "ENCODING SPEED", true)
	a.label("videoAudioLabel", "VIDEO AUDIO BITRATE", true)
	a.combo("preset", []string{"Ultra fast", "Fast / balanced", "Slower / smaller file"}, 1)
	a.combo("videoAudio", []string{"128 kbps", "192 kbps", "256 kbps", "320 kbps"}, 1)
	a.label("audioBitrateLabel", "MP3 BITRATE", true)
	a.label("sampleRateLabel", "SAMPLE RATE", true)
	a.label("channelsLabel", "CHANNELS", true)
	a.combo("bitrate", []string{"128 kbps", "192 kbps", "256 kbps", "320 kbps"}, 1)
	a.combo("sampleRate", []string{"Original / automatic", "44.1 kHz", "48 kHz"}, 0)
	a.combo("channels", []string{"Stereo", "Mono"}, 0)
	a.add("openSource", "BUTTON", "Open source", winui.WS_TABSTOP, 0, false)
	a.add("preview", "BUTTON", "30s preview", winui.WS_TABSTOP, 0, false)
	a.add("openResult", "BUTTON", "Open result", winui.WS_TABSTOP, 0, false)
	a.add("convert", "BUTTON", "Start conversion", winui.WS_TABSTOP, 0, true)
	a.add("cancel", "BUTTON", "Cancel", winui.WS_TABSTOP, 0, false)
	a.label("status", "Choose a source file to get started.", false)
	a.add("progress", "msctls_progress32", "", 0, 0, false)
	winui.Send(a.widgets["progress"], winui.PBM_SETRANGE32, 0, 100)
	winui.BlueProgress(a.widgets["progress"])
	a.label("elapsed", "Elapsed: 0:00", false)
	a.label("logLabel", "ACTIVITY LOG", true)
	a.add("log", "EDIT", "", winui.ES_MULTILINE|winui.ES_READONLY|winui.ES_AUTOVSCROLL|winui.WS_VSCROLL|winui.WS_BORDER, winui.WS_EX_CLIENTEDGE, false)
	a.add("foot", "BUTTON", "© by www.perplex.click", winui.WS_TABSTOP, 0, false)
	a.add("license", "BUTTON", "Licenses / About", winui.WS_TABSTOP, 0, false)
	a.add("clearTemps", "BUTTON", "Clear previews", winui.WS_TABSTOP, 0, false)
	a.add("clearInput", "BUTTON", "Remove source", winui.WS_TABSTOP, 0, false)
}
func (a *App) place(name string, x, y, w, h int) { winui.Move(a.widgets[name], x, y, w, h) }
func (a *App) layout() {
	if a.main == 0 || len(a.widgets) == 0 {
		return
	}
	width, height := winui.Client(a.main)
	if width < 970 {
		width = 970
	}
	if height < 735 {
		height = 735
	}
	margin := 34
	full := width - margin*2
	gap := 14
	a.place("title", 106, 17, width-347, 38)
	a.place("subtitle", 107, 56, width-347, 20)
	a.place("heroBadge", width-188, 31, 155, 26)
	a.place("inputLabel", margin, 102, full, 19)
	a.place("input", margin, 126, full-151, 28)
	a.place("inputBrowse", width-margin-143, 124, 143, 32)
	a.place("fileInfo", margin, 159, full, 19)
	left := (full - gap) * 40 / 100
	right := full - left - gap
	a.place("audioModeLabel", margin, 191, left, 19)
	a.place("trackLabel", margin+left+gap, 191, right, 19)
	a.place("audioMode", margin, 211, left, 220)
	a.place("track", margin+left+gap, 211, right, 270)
	a.place("externalLabel", margin, 246, full, 18)
	a.place("external", margin, 265, full-151, 28)
	a.place("externalBrowse", width-margin-143, 263, 143, 32)
	formLeft := full / 3
	formRight := full - formLeft - gap
	a.place("formatLabel", margin, 312, formLeft, 20)
	a.place("outputLabel", margin+formLeft+gap, 312, formRight, 20)
	a.place("format", margin, 334, formLeft, 220)
	a.place("output", margin+formLeft+gap, 334, formRight-135, 28)
	a.place("outputBrowse", width-margin-128, 332, 128, 32)
	a.place("settingsLabel", margin, 373, full, 20)
	col := (full - 2*gap) / 3
	labels := []string{"modeLabel", "qualityLabel", "resLabel"}
	fields := []string{"mode", "quality", "resolution"}
	for i := 0; i < 3; i++ {
		x := margin + i*(col+gap)
		a.place(labels[i], x, 396, col, 18)
		a.place(fields[i], x, 415, col, 220)
	}
	half := (full - gap) / 2
	a.place("presetLabel", margin, 448, half, 18)
	a.place("videoAudioLabel", margin+half+gap, 448, half, 18)
	a.place("preset", margin, 468, half, 200)
	a.place("videoAudio", margin+half+gap, 468, half, 200)
	labels = []string{"audioBitrateLabel", "sampleRateLabel", "channelsLabel"}
	fields = []string{"bitrate", "sampleRate", "channels"}
	for i := 0; i < 3; i++ {
		x := margin + i*(col+gap)
		a.place(labels[i], x, 396, col, 18)
		a.place(fields[i], x, 415, col, 220)
	}
	btnY := 510
	btnW := (full - 4*gap) / 5
	for i, k := range []string{"openSource", "preview", "openResult", "convert", "cancel"} {
		a.place(k, margin+i*(btnW+gap), btnY, btnW, 34)
	}
	a.place("status", margin, 558, full-145, 21)
	a.place("elapsed", width-margin-140, 558, 140, 21)
	a.place("progress", margin, 584, full, 13)
	a.place("logLabel", margin, 618, full, 20)
	logHeight := height - 700
	if logHeight < 30 {
		logHeight = 30
	}
	a.place("log", margin, 641, full, logHeight)
	a.place("foot", margin-10, height-33, 180, 28)
	a.place("clearTemps", margin+184, height-33, 145, 28)
	a.place("clearInput", margin+335, height-33, 145, 28)
	a.place("license", width-margin-170, height-33, 170, 28)
}
func (a *App) draw() {
	width, height := winui.Client(a.main)
	if width < 970 {
		width = 970
	}
	if height < 735 {
		height = 735
	}
	card := winui.RGB(255, 255, 255)
	outline := winui.RGB(213, 228, 247)
	panels := []winui.Panel{
		{X: 18, Y: 90, W: width - 36, H: 88, Radius: 20, Fill: card, Border: outline},
		{X: 18, Y: 183, W: width - 36, H: 114, Radius: 20, Fill: card, Border: outline},
		{X: 18, Y: 302, W: width - 36, H: 196, Radius: 20, Fill: card, Border: outline},
		{X: 18, Y: 552, W: width - 36, H: 54, Radius: 18, Fill: card, Border: outline},
		{X: 18, Y: 611, W: width - 36, H: height - 653, Radius: 20, Fill: card, Border: outline},
	}
	winui.PaintPanels(a.main, 84, panels, a.logo)
}
func (a *App) paintControl(dc, child uintptr, edit bool) uintptr {
	w := winui.HWND(child)
	if edit || w == a.widgets["input"] || w == a.widgets["external"] || w == a.widgets["log"] {
		return winui.EditColor(dc, winui.RGB(28, 47, 78))
	}
	name := ""
	for k, h := range a.widgets {
		if h == w {
			name = k
			break
		}
	}
	color := winui.RGB(40, 61, 91)
	switch name {
	case "title":
		color = winui.RGB(255, 255, 255)
	case "subtitle", "heroBadge":
		color = winui.RGB(189, 224, 255)
	case "inputLabel", "audioModeLabel", "trackLabel", "externalLabel", "formatLabel", "outputLabel", "settingsLabel", "modeLabel", "qualityLabel", "resLabel", "presetLabel", "videoAudioLabel", "audioBitrateLabel", "sampleRateLabel", "channelsLabel", "logLabel":
		color = winui.RGB(28, 93, 170)
	case "fileInfo", "status", "elapsed":
		color = winui.RGB(81, 110, 150)
	}
	background := winui.RGB(255, 255, 255)
	if name == "title" || name == "subtitle" || name == "heroBadge" {
		background = winui.RGB(12, 37, 78)
	}
	if name == "status" || name == "elapsed" || name == "foot" {
		background = winui.RGB(255, 255, 255)
	}
	return winui.LabelColor(dc, color, background)
}
func (a *App) drawButton(id int, param uintptr) {
	name := a.ids[id]
	bg, fg, border := winui.RGB(255, 255, 255), winui.RGB(33, 90, 166), winui.RGB(190, 213, 239)
	switch name {
	case "convert":
		bg, fg, border = winui.RGB(26, 105, 218), winui.RGB(255, 255, 255), winui.RGB(26, 105, 218)
	case "preview":
		bg, fg, border = winui.RGB(215, 241, 255), winui.RGB(18, 90, 156), winui.RGB(158, 222, 249)
	case "inputBrowse", "externalBrowse", "outputBrowse":
		bg, fg, border = winui.RGB(229, 240, 255), winui.RGB(23, 91, 177), winui.RGB(207, 226, 249)
	case "cancel":
		bg, fg, border = winui.RGB(255, 247, 248), winui.RGB(156, 62, 86), winui.RGB(244, 217, 223)
	case "foot", "license", "clearTemps", "clearInput":
		bg, fg, border = winui.RGB(239, 245, 253), winui.RGB(50, 99, 154), winui.RGB(239, 245, 253)
	}
	winui.DrawButton(param, bg, fg, border)
}
func (a *App) setStatus(s string) { winui.SetText(a.widgets["status"], s) }
func (a *App) log(s string) {
	w := a.widgets["log"]
	winui.Send(w, winui.EM_SETSEL, ^uintptr(0), ^uintptr(0))
	winui.Send(w, winui.EM_REPLACESEL, 0, uintptr(unsafe.Pointer(winui.Ptr(s+"\r\n"))))
}
func (a *App) command(id, notify int) {
	name := a.ids[id]
	if notify == winui.CBN_SELCHANGE {
		switch name {
		case "format":
			a.changedTarget()
		case "audioMode":
			a.updateTracks()
			a.updateEnabled()
		case "mode":
			a.updateEnabled()
		}
		return
	}
	if notify != winui.BN_CLICKED {
		return
	}
	switch name {
	case "inputBrowse":
		path, err := winui.Dialog(a.main, false, "", "Select a video or audio file", winui.MediaFilter())
		if err != nil {
			winui.Msg(a.main, "File picker error", err.Error()+"\n\nYou can also drag a media file into the application window.", winui.MB_ICONWARNING)
			return
		}
		if path != "" {
			a.chooseInput(path)
		}
	case "externalBrowse":
		path, err := winui.Dialog(a.main, false, "", "Select a separate audio track", winui.ExternalFilter())
		if err != nil {
			winui.Msg(a.main, "Audio picker error", err.Error(), winui.MB_ICONWARNING)
			return
		}
		if path != "" {
			a.chooseExternal(path)
		}
	case "outputBrowse":
		if len(a.targetList) == 0 {
			return
		}
		target := a.target()
		path, err := winui.Dialog(a.main, true, winui.GetText(a.widgets["output"]), "Save output file", winui.OutputFilter(target))
		if err != nil {
			winui.Msg(a.main, "Save dialog error", err.Error(), winui.MB_ICONWARNING)
			return
		}
		if path != "" {
			if engine.Extension(path) != target {
				path = strings.TrimSuffix(path, filepath.Ext(path)) + "." + target
			}
			winui.SetText(a.widgets["output"], path)
		}
	case "convert":
		a.convert(false)
	case "preview":
		a.convert(true)
	case "cancel":
		if a.cancel != nil {
			a.setStatus("Cancelling …")
			a.cancel()
		}
	case "openSource":
		if a.currentInput != "" {
			a.open(a.currentInput)
		}
	case "openResult":
		if a.lastResult != "" {
			a.open(a.lastResult)
		}
	case "clearTemps":
		a.cleanupPreview()
		a.setStatus("Temporary previews removed (unless a media player is still using them).")
	case "clearInput":
		a.resetInput()
	case "foot":
		a.open("https://www.perplex.click/")
	case "license":
		winui.Msg(a.main, "Perplex Media Converter 1.1.6", "Application: MIT License, Copyright (c) 2026 Michael Lappenbusch.\n\nBundled FFmpeg / FFprobe: GPL version 3 or later; separate command-line programs. Includes x264 and LAME.\n\nThe full licenses are in the licenses folder. The matching FFmpeg source archive is offered beside the installer on the download page. See FFMPEG-SOURCE.txt.\n\nThe third-party notice will open next.", winui.MB_OK|winui.MB_ICONINFORMATION)
		exe, _ := os.Executable()
		path := filepath.Join(filepath.Dir(exe), "THIRD_PARTY.txt")
		if _, err := os.Stat(path); err == nil {
			a.open(path)
		} else {
			winui.Msg(a.main, "MIT license", "Copyright (c) 2026 Michael Lappenbusch\n\nSee the included LICENSE.txt for the full MIT license.", winui.MB_OK|winui.MB_ICONINFORMATION)
		}
	}
}
func (a *App) open(path string) {
	if err := winui.Open(path); err != nil {
		winui.Msg(a.main, "Open file", err.Error(), winui.MB_OK|winui.MB_ICONWARNING)
	}
}
func (a *App) resetInput() {
	if a.busy {
		return
	}
	a.genInput++
	a.genExternal++
	a.currentInput = ""
	a.currentExternal = ""
	a.info = nil
	a.externalInfo = nil
	a.lastResult = ""
	a.targetList = nil
	a.trackIndices = nil
	a.cleanupPreview()
	winui.SetText(a.widgets["input"], "")
	winui.SetText(a.widgets["external"], "")
	winui.SetText(a.widgets["output"], "")
	winui.SetText(a.widgets["fileInfo"], "Drop a file here, or browse  ·  MKV, MP4, AVI, MOV, WebM, FLAC, WAV, MP3, M4A")
	winui.ResetCombo(a.widgets["format"])
	winui.AddCombo(a.widgets["format"], "Select a source first")
	winui.SelectCombo(a.widgets["format"], 0)
	winui.SelectCombo(a.widgets["audioMode"], 0)
	a.updateTracks()
	winui.Send(a.widgets["progress"], winui.PBM_SETPOS, 0, 0)
	winui.SetText(a.widgets["log"], "")
	a.setStatus("Ready for a new file.")
	a.updateEnabled()
}
func (a *App) chooseInput(path string) {
	if a.busy {
		return
	}
	if len(engine.Targets(path)) == 0 {
		winui.Msg(a.main, "Unsupported file", "Choose an MKV, MP4, AVI, MOV, WebM, FLAC, WAV, MP3, or M4A file.", winui.MB_OK|winui.MB_ICONWARNING)
		return
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() == 0 {
		winui.Msg(a.main, "Invalid file", "The source file does not exist or is empty.", winui.MB_OK|winui.MB_ICONERROR)
		return
	}
	a.genInput++
	a.currentInput = path
	a.info = nil
	a.lastResult = ""
	a.cleanupPreview()
	winui.SetText(a.widgets["input"], path)
	winui.SetText(a.widgets["fileInfo"], fmt.Sprintf("%s  ·  %.1f MB  ·  Analyzing audio/video streams …", strings.ToUpper(engine.Extension(path)), float64(fi.Size())/1024/1024))
	a.targetList = engine.Targets(path)
	winui.ResetCombo(a.widgets["format"])
	for _, target := range a.targetList {
		winui.AddCombo(a.widgets["format"], strings.ToUpper(target))
	}
	winui.SelectCombo(a.widgets["format"], 0)
	if engine.Extension(path) == "mp4" {
		winui.SelectCombo(a.widgets["mode"], 1)
	} else {
		winui.SelectCombo(a.widgets["mode"], 0)
	}
	winui.SelectCombo(a.widgets["audioMode"], 0)
	a.changedTarget()
	a.updateTracks()
	a.updateEnabled()
	a.setStatus("Analyzing source file …")
	gen := a.genInput
	probeExe := a.ffprobe
	if probeExe == "" {
		a.setStatus("Please install FFmpeg and FFprobe first.")
		return
	}
	go func() {
		info, err := engine.Probe(probeExe, path)
		a.emit(appEvent{kind: "probeInput", path: path, gen: gen, info: info, err: err})
	}()
}
func (a *App) chooseExternal(path string) {
	if a.busy {
		return
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() == 0 {
		winui.Msg(a.main, "Invalid file", "The external audio file does not exist or is empty.", winui.MB_OK|winui.MB_ICONERROR)
		return
	}
	a.genExternal++
	a.currentExternal = path
	a.externalInfo = nil
	a.cleanupPreview()
	winui.SetText(a.widgets["external"], path)
	winui.SelectCombo(a.widgets["audioMode"], 1)
	a.updateTracks()
	a.updateEnabled()
	a.setStatus("Analyzing external audio track …")
	gen := a.genExternal
	if a.ffprobe == "" {
		return
	}
	go func() {
		info, err := engine.Probe(a.ffprobe, path)
		a.emit(appEvent{kind: "probeExternal", path: path, gen: gen, info: info, err: err})
	}()
}
func (a *App) changedTarget() {
	t := a.target()
	if t == "" {
		return
	}
	if a.currentInput != "" {
		base := strings.TrimSuffix(a.currentInput, filepath.Ext(a.currentInput))
		winui.SetText(a.widgets["output"], base+"_converted."+t)
	}
	a.updateEnabled()
}
func (a *App) target() string {
	i := winui.SelectedCombo(a.widgets["format"])
	if i < 0 || i >= len(a.targetList) {
		return ""
	}
	return a.targetList[i]
}
func (a *App) audioMode() engine.AudioMode {
	switch winui.SelectedCombo(a.widgets["audioMode"]) {
	case 1:
		return engine.AudioExternal
	case 2:
		return engine.AudioNone
	default:
		return engine.AudioEmbedded
	}
}
func (a *App) updateTracks() {
	a.trackIndices = nil
	winui.ResetCombo(a.widgets["track"])
	mode := a.audioMode()
	if mode == engine.AudioNone {
		winui.AddCombo(a.widgets["track"], "No audio track")
		winui.SelectCombo(a.widgets["track"], 0)
		return
	}
	var info *engine.MediaInfo
	if mode == engine.AudioExternal {
		info = a.externalInfo
	} else {
		info = a.info
	}
	if info == nil {
		winui.AddCombo(a.widgets["track"], "Audio track information not available yet")
		winui.SelectCombo(a.widgets["track"], 0)
		return
	}
	tracks := info.AudioTracks()
	if len(tracks) == 0 {
		winui.AddCombo(a.widgets["track"], "No audio tracks found")
		winui.SelectCombo(a.widgets["track"], 0)
		return
	}
	sel := 0
	for i, s := range tracks {
		a.trackIndices = append(a.trackIndices, s.Index)
		winui.AddCombo(a.widgets["track"], s.Label())
		if s.Disposition.Default == 1 {
			sel = i
		}
	}
	winui.SelectCombo(a.widgets["track"], sel)
}
func (a *App) updateEnabled() {
	audioTarget := a.target() == "mp3" || a.target() == "wav"
	videoTarget := a.target() == "mp4" || a.target() == "mkv"
	copyMode := winui.SelectedCombo(a.widgets["mode"]) == 1
	// Keep the interface focused: show only settings relevant to the target format.
	for _, k := range []string{"modeLabel", "qualityLabel", "resLabel", "presetLabel", "videoAudioLabel", "mode", "quality", "resolution", "preset", "videoAudio"} {
		winui.ShowControl(a.widgets[k], videoTarget)
	}
	for _, k := range []string{"audioBitrateLabel", "sampleRateLabel", "channelsLabel", "bitrate", "sampleRate", "channels"} {
		winui.ShowControl(a.widgets[k], audioTarget)
	}
	if videoTarget {
		winui.SetText(a.widgets["settingsLabel"], "VIDEO SETTINGS  /  H.264 + AAC")
	}
	if audioTarget {
		winui.SetText(a.widgets["settingsLabel"], "AUDIO SETTINGS  /  MP3 + WAV")
	}
	active := a.currentInput != "" && !a.busy
	for _, k := range []string{"externalBrowse", "outputBrowse", "format", "audioMode", "track", "output", "mode", "quality", "resolution", "preset", "videoAudio", "bitrate", "sampleRate", "channels", "openSource", "preview", "openResult", "convert"} {
		winui.Enable(a.widgets[k], active)
	}
	winui.Enable(a.widgets["inputBrowse"], !a.busy)
	winui.Enable(a.widgets["mode"], active && videoTarget)
	for _, k := range []string{"quality", "resolution", "preset", "videoAudio"} {
		winui.Enable(a.widgets[k], active && videoTarget && !copyMode)
	}
	for _, k := range []string{"bitrate", "sampleRate", "channels"} {
		winui.Enable(a.widgets[k], active && audioTarget)
	}
	winui.Enable(a.widgets["externalBrowse"], active && a.audioMode() == engine.AudioExternal)
	winui.Enable(a.widgets["external"], active && a.audioMode() == engine.AudioExternal)
	winui.Enable(a.widgets["track"], active && a.audioMode() != engine.AudioNone)
	hasVideo := false
	if a.info != nil {
		_, hasVideo = a.info.FirstVideo()
	}
	winui.Enable(a.widgets["preview"], active && hasVideo)
	winui.Enable(a.widgets["openResult"], active && a.lastResult != "")
	winui.Enable(a.widgets["convert"], active && a.ready && a.info != nil)
	winui.Enable(a.widgets["cancel"], a.busy)
	winui.Enable(a.widgets["clearInput"], active)
	winui.Enable(a.widgets["clearTemps"], !a.busy)
}
func selectedIndex(w winui.HWND, values []string, defaultValue string) string {
	idx := winui.SelectedCombo(w)
	if idx < 0 || idx >= len(values) {
		return defaultValue
	}
	return values[idx]
}
func (a *App) options() (engine.Options, error) {
	t := a.target()
	o := engine.Defaults(a.currentInput, t)
	o.Output = winui.GetText(a.widgets["output"])
	if o.Output == "" {
		return o, errors.New("Please choose an output path.")
	}
	if !strings.EqualFold(engine.Extension(o.Output), t) {
		return o, fmt.Errorf("The output filename must end with .%s.", t)
	}
	o.AudioMode = a.audioMode()
	o.ExternalFile = a.currentExternal
	o.EmbeddedTrack = -1
	o.ExternalTrack = -1
	idx := winui.SelectedCombo(a.widgets["track"])
	if idx >= 0 && idx < len(a.trackIndices) {
		if o.AudioMode == engine.AudioEmbedded {
			o.EmbeddedTrack = a.trackIndices[idx]
		} else {
			o.ExternalTrack = a.trackIndices[idx]
		}
	}
	o.Mode = selectedIndex(a.widgets["mode"], []string{"encode", "copy"}, "encode")
	o.Quality = selectedIndex(a.widgets["quality"], []string{"18", "23", "28"}, "23")
	o.Resolution = selectedIndex(a.widgets["resolution"], []string{"original", "1080", "720", "480"}, "original")
	o.Preset = selectedIndex(a.widgets["preset"], []string{"ultrafast", "veryfast", "medium"}, "veryfast")
	o.VideoAudioBitrate = 128 + 64*winui.SelectedCombo(a.widgets["videoAudio"])
	o.AudioBitrate = 128 + 64*winui.SelectedCombo(a.widgets["bitrate"])
	o.SampleRate = selectedIndex(a.widgets["sampleRate"], []string{"original", "44100", "48000"}, "original")
	o.Channels = selectedInt(a.widgets["channels"], []int{2, 1}, 2)
	if o.AudioMode == engine.AudioNone && (t == "mp3" || t == "wav") {
		return o, errors.New("Please select an audio track for audio exports.")
	}
	if o.AudioMode == engine.AudioExternal && a.externalInfo == nil {
		return o, errors.New("Please choose a valid external audio file first.")
	}
	if err := engine.CheckPaths(o); err != nil {
		return o, err
	}
	return o, nil
}
func selectedInt(w winui.HWND, values []int, def int) int {
	i := winui.SelectedCombo(w)
	if i < 0 || i >= len(values) {
		return def
	}
	return values[i]
}
func (a *App) convert(preview bool) {
	if a.busy || !a.ready || a.info == nil {
		return
	}
	opts, err := a.options()
	if err != nil {
		winui.Msg(a.main, "Check settings", err.Error(), winui.MB_OK|winui.MB_ICONWARNING)
		return
	}
	if preview {
		if _, ok := a.info.FirstVideo(); !ok {
			return
		}
	}
	if !preview {
		if _, err := os.Stat(opts.Output); err == nil {
			if winui.Msg(a.main, "Replace existing file?", "The output file already exists. Replace it only AFTER a successful conversion?", winui.MB_YESNO|winui.MB_ICONQUESTION) != winui.IDYES {
				return
			}
		}
	}
	src := *a.info
	var external *engine.MediaInfo
	if a.externalInfo != nil {
		copy := *a.externalInfo
		external = &copy
	}
	dest := opts.Output
	if preview {
		dest = filepath.Join(os.TempDir(), fmt.Sprintf("perplex-preview-%d.mp4", time.Now().UnixNano()))
	}
	temp := dest
	if !preview {
		temp = filepath.Join(filepath.Dir(dest), fmt.Sprintf(".%s.perplex-part-%d.%s", filepath.Base(strings.TrimSuffix(dest, filepath.Ext(dest))), time.Now().UnixNano(), opts.Target))
	}
	opts.Output = temp
	var args []string
	if preview {
		args, err = engine.PreviewArgs(opts, src, external, temp)
	} else {
		args, err = engine.BuildArgs(opts, src, external)
	}
	if err != nil {
		winui.Msg(a.main, "Cannot convert", err.Error(), winui.MB_OK|winui.MB_ICONWARNING)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.busy = true
	a.started = time.Now()
	a.updateEnabled()
	winui.Timer(a.main, 1, 1000)
	winui.Send(a.widgets["progress"], winui.PBM_SETPOS, 0, 0)
	winui.SetText(a.widgets["elapsed"], "Elapsed: 0:00")
	winui.SetText(a.widgets["log"], "")
	if preview {
		a.setStatus("Creating a 30-second preview (H.264/AAC) …")
	} else {
		a.setStatus("Conversion in progress …")
	}
	a.log("Processed locally with FFmpeg. No files are uploaded.")
	a.log("Source: " + opts.Input)
	if opts.AudioMode == engine.AudioExternal {
		a.log("Separate audio: " + opts.ExternalFile + " (replaces original track)")
	}
	a.log("Audio source: " + string(opts.AudioMode))
	a.log("Output: " + dest)
	progressDuration := src.DurationSeconds()
	if preview && progressDuration > 30 {
		progressDuration = 30
	}
	go a.run(ctx, args, temp, dest, progressDuration, preview)
}
func durationDisplay(d time.Duration) string {
	sec := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}
func (a *App) run(ctx context.Context, args []string, temp, dest string, duration float64, preview bool) {
	defer func() {
		if ctx.Err() != nil {
			_ = os.Remove(temp)
		}
	}()
	cmd := exec.CommandContext(ctx, a.ffmpeg, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.emit(appEvent{kind: "done", err: err})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.emit(appEvent{kind: "done", err: err})
		return
	}
	if err = cmd.Start(); err != nil {
		a.emit(appEvent{kind: "done", err: err})
		return
	}
	logDone := make(chan string, 1)
	go func() {
		scan := bufio.NewScanner(stderr)
		scan.Buffer(make([]byte, 8192), 1024*1024)
		last := ""
		for scan.Scan() {
			line := scan.Text()
			if len(last) > 2400 {
				last = last[len(last)-1600:]
			}
			last += "\n" + line
			a.emit(appEvent{kind: "log", text: line})
		}
		logDone <- last
	}()
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 1024), 1024*1024)
	lastPercent := -1
	for scan.Scan() {
		line := scan.Text()
		if duration > 0 && (strings.HasPrefix(line, "out_time_ms=") || strings.HasPrefix(line, "out_time_us=")) {
			parts := strings.SplitN(line, "=", 2)
			microseconds, _ := strconv.ParseFloat(parts[1], 64)
			pct := int(microseconds / 1000000 / duration * 100)
			if pct > 99 {
				pct = 99
			}
			if pct < 0 {
				pct = 0
			}
			if pct != lastPercent {
				lastPercent = pct
				a.emit(appEvent{kind: "progress", percent: pct})
			}
		}
	}
	cmdErr := cmd.Wait()
	lastLog := <-logDone
	if ctx.Err() != nil {
		_ = os.Remove(temp)
		a.emit(appEvent{kind: "done", err: errors.New("Job cancelled.")})
		return
	}
	if cmdErr != nil {
		_ = os.Remove(temp)
		if len(lastLog) > 1250 {
			lastLog = lastLog[len(lastLog)-1250:]
		}
		a.emit(appEvent{kind: "done", err: fmt.Errorf("FFmpeg error: %w\n%s", cmdErr, strings.TrimSpace(lastLog))})
		return
	}
	fi, err := os.Stat(temp)
	if err != nil || fi.Size() == 0 {
		_ = os.Remove(temp)
		a.emit(appEvent{kind: "done", err: errors.New("FFmpeg did not produce a complete output file.")})
		return
	}
	result, err := engine.Probe(a.ffprobe, temp)
	if err != nil {
		_ = os.Remove(temp)
		a.emit(appEvent{kind: "done", err: fmt.Errorf("Output file is invalid: %w", err)})
		return
	}
	required := "video"
	if !preview && (engine.Extension(dest) == "mp3" || engine.Extension(dest) == "wav") {
		required = "audio"
	}
	valid := false
	for _, s := range result.Streams {
		if s.CodecType == required {
			valid = true
		}
	}
	if !valid {
		_ = os.Remove(temp)
		a.emit(appEvent{kind: "done", err: fmt.Errorf("Output does not contain a valid %s stream", required)})
		return
	}
	if !preview {
		if err := winui.ReplaceFile(temp, dest); err != nil {
			_ = os.Remove(temp)
			a.emit(appEvent{kind: "done", err: err})
			return
		}
	}
	a.emit(appEvent{kind: "done", output: dest, percent: 100, text: fmt.Sprintf("Done: %.1f MB  ·  %d audio track(s)", float64(fi.Size())/1024/1024, len(result.AudioTracks())), path: func() string {
		if preview {
			return "preview"
		}
		return "conversion"
	}()})
}
func (a *App) handleEvents() {
	for count := 0; count < 200; count++ {
		var ev appEvent
		select {
		case ev = <-a.events:
		default:
			return
		}
		switch ev.kind {
		case "probeInput":
			if ev.gen != a.genInput || ev.path != a.currentInput {
				continue
			}
			if ev.err != nil {
				a.info = nil
				a.setStatus("Unable to read source: " + ev.err.Error())
				winui.Msg(a.main, "FFprobe could not read this file", ev.err.Error(), winui.MB_OK|winui.MB_ICONERROR)
			} else {
				a.info = &ev.info
				fi, _ := os.Stat(ev.path)
				size := float64(0)
				if fi != nil {
					size = float64(fi.Size()) / 1024 / 1024
				}
				v, hasVideo := ev.info.FirstVideo()
				extra := "Audio"
				if hasVideo {
					extra = fmt.Sprintf("%dx%d", v.Width, v.Height)
				}
				winui.SetText(a.widgets["fileInfo"], fmt.Sprintf("%s  ·  %.1f MB  ·  %s  ·  %s  ·  %d audio track(s)", strings.ToUpper(engine.Extension(ev.path)), size, durationDisplay(time.Duration(ev.info.DurationSeconds()*float64(time.Second))), extra, len(ev.info.AudioTracks())))
				if len(ev.info.AudioTracks()) == 0 && hasVideo {
					winui.SelectCombo(a.widgets["audioMode"], 2)
				}
				a.updateTracks()
				a.setStatus("Source analyzed. Choose an audio track and output settings.")
			}
			a.updateEnabled()
		case "probeExternal":
			if ev.gen != a.genExternal || ev.path != a.currentExternal {
				continue
			}
			if ev.err != nil || len(ev.info.AudioTracks()) == 0 {
				a.externalInfo = nil
				msg := "No audio streams found in the separate file."
				if ev.err != nil {
					msg = ev.err.Error()
				}
				a.setStatus("Cannot use this audio file.")
				winui.Msg(a.main, "Separate audio", msg, winui.MB_OK|winui.MB_ICONERROR)
			} else {
				a.externalInfo = &ev.info
				a.setStatus(fmt.Sprintf("Separate file analyzed: %d audio track(s) available.", len(ev.info.AudioTracks())))
			}
			a.updateTracks()
			a.updateEnabled()
		case "progress":
			winui.Send(a.widgets["progress"], winui.PBM_SETPOS, uintptr(ev.percent), 0)
			a.setStatus(fmt.Sprintf("Processing  ·  %d %%", ev.percent))
		case "log":
			a.log(ev.text)
		case "done":
			a.busy = false
			if a.cancel != nil {
				a.cancel()
				a.cancel = nil
			}
			winui.StopTimer(a.main, 1)
			if ev.err != nil {
				a.setStatus("Not completed: " + strings.Split(ev.err.Error(), "\n")[0])
				a.log("ERROR: " + ev.err.Error())
				if !strings.Contains(ev.err.Error(), "Vorgang abgebrochen") {
					winui.Msg(a.main, "Conversion failed", ev.err.Error()+"\n\nFor incompatible codecs, choose Re-encode mode.", winui.MB_OK|winui.MB_ICONERROR)
				}
			} else {
				winui.Send(a.widgets["progress"], winui.PBM_SETPOS, 100, 0)
				a.setStatus(ev.text)
				a.log(ev.text)
				if ev.path == "preview" {
					a.previewFiles = append(a.previewFiles, ev.output)
					a.open(ev.output)
				} else {
					a.lastResult = ev.output
					winui.Msg(a.main, "Conversion completed", "File saved to:\n"+ev.output, winui.MB_OK|winui.MB_ICONINFORMATION)
				}
			}
			a.updateEnabled()
		}
	}
	if len(a.events) > 0 {
		winui.Post(a.main, eventMessage, 0, 0)
	}
}
func (a *App) cleanupPreview() {
	keep := make([]string, 0)
	for _, p := range a.previewFiles {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			keep = append(keep, p)
		}
	}
	a.previewFiles = keep
}

// Required so older Windows systems never display a console for GUI processes.
var _ io.Reader

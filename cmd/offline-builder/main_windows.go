//go:build windows

package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"perplex-media-converter/internal/offlinebundle"
	"perplex-media-converter/internal/winui"
)

//go:embed payload/*
var payload embed.FS

const title = "Perplex Media Converter — Offline Setup Builder"
const wmEvent = winui.WM_APP + 47

type state struct {
	main                                                                                      winui.HWND
	heading, sub, source, target, information, progress, pct, build, choose, closeBtn, footer winui.HWND
	font, headFont, logo                                                                      uintptr
	busy, complete                                                                            bool
	events                                                                                    chan event
	cancel                                                                                    context.CancelFunc
	out                                                                                       string
	ffmpegPath, ffprobePath                                                                   string
}
type event struct {
	text string
	pct  int
	done bool
	err  error
}

var ui *state

func wndproc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	s := ui
	if s != nil && winui.HWND(hwnd) == s.main && s.main != 0 {
		switch msg {
		case winui.WM_ERASEBKGND:
			return 1
		case winui.WM_PAINT:
			s.draw()
			return 0
		case winui.WM_SIZE:
			s.layout()
			return 0
		case winui.WM_CTLCOLORSTATIC:
			if winui.HWND(lp) == s.heading {
				return winui.LabelColor(wp, winui.RGB(255, 255, 255), winui.RGB(12, 37, 78))
			}
			if winui.HWND(lp) == s.sub {
				return winui.LabelColor(wp, winui.RGB(187, 223, 255), winui.RGB(12, 37, 78))
			}
			if winui.HWND(lp) == s.footer {
				return winui.LabelColor(wp, winui.RGB(69, 105, 153), winui.RGB(239, 245, 253))
			}
			return winui.LabelColor(wp, winui.RGB(37, 67, 111), winui.RGB(255, 255, 255))
		case winui.WM_DRAWITEM:
			if int(wp) == 1001 {
				winui.DrawButton(lp, winui.RGB(26, 105, 218), winui.RGB(255, 255, 255), winui.RGB(26, 105, 218))
			} else {
				winui.DrawButton(lp, winui.RGB(236, 244, 255), winui.RGB(29, 90, 167), winui.RGB(203, 221, 244))
			}
			return 1
		case winui.WM_COMMAND:
			switch int(wp & 0xffff) {
			case 1001:
				if !s.busy {
					s.begin()
				}
			case 1003:
				if !s.busy {
					s.chooseTools()
				}
			case 1002:
				if s.busy {
					if s.cancel != nil {
						s.cancel()
					}
				} else {
					winui.Destroy(s.main)
				}
			}
			return 0
		case wmEvent:
			s.receive()
			return 0
		case winui.WM_CLOSE:
			if s.busy {
				if winui.Msg(s.main, "Cancel packaging?", "Cancel offline setup creation?", winui.MB_YESNO|winui.MB_ICONQUESTION) != winui.IDYES {
					return 0
				}
				if s.cancel != nil {
					s.cancel()
				}
			}
			winui.Destroy(s.main)
			return 0
		case winui.WM_DESTROY:
			if s.cancel != nil {
				s.cancel()
			}
			winui.FreeIcon(s.logo)
			winui.PostQuit(0)
			return 0
		}
	}
	return winui.DefWndProc(winui.HWND(hwnd), msg, wp, lp)
}

func main() {
	runtime.LockOSThread()
	winui.CommonControls()
	ui = &state{events: make(chan event, 128), out: outputPath()}
	if err := winui.Register("PerplexOfflineBuilder114", syscall.NewCallback(wndproc)); err != nil {
		winui.Msg(0, title, err.Error(), winui.MB_ICONERROR)
		return
	}
	h, err := winui.CreateMain("PerplexOfflineBuilder114", title, 820, 465)
	if err != nil {
		winui.Msg(0, title, err.Error(), winui.MB_ICONERROR)
		return
	}
	ui.main = h
	ui.logo = loadIcon()
	winui.SetIconFromHandle(h, ui.logo)
	ui.font = winui.Font(16, false)
	ui.headFont = winui.Font(26, true)
	ui.heading = ctrl("STATIC", "Perplex Media Converter", 0, 1010)
	ui.sub = ctrl("STATIC", "OFFLINE INSTALLER BUILDER  ·  WINDOWS 10 / 11  ·  v1.1.6", 0, 1011)
	ui.information = ctrl("STATIC", "Create a complete Windows Setup with FFmpeg included. No download during installation.", 0, 1012)
	ui.source = ctrl("STATIC", "FFmpeg source: "+ui.previewSource(), 0, 1013)
	ui.target = ctrl("STATIC", "Output: "+ui.out, 0, 1014)
	ui.progress = ctrl("msctls_progress32", "", 0, 1015)
	ui.pct = ctrl("STATIC", "0%", 0, 1016)
	ui.build = ctrl("BUTTON", "Create offline setup", winui.WS_TABSTOP, 1001)
	ui.choose = ctrl("BUTTON", "Choose FFmpeg...", winui.WS_TABSTOP, 1003)
	ui.closeBtn = ctrl("BUTTON", "Close", winui.WS_TABSTOP, 1002)
	ui.footer = ctrl("STATIC", "© by www.perplex.click  ·  MIT © Michael Lappenbusch  ·  FFmpeg GPLv3", 0, 1017)
	winui.SetFont(ui.heading, ui.headFont)
	for _, x := range []winui.HWND{ui.sub, ui.information, ui.source, ui.target, ui.pct, ui.build, ui.choose, ui.closeBtn, ui.footer} {
		winui.SetFont(x, ui.font)
	}
	ui.layout()
	winui.Send(ui.progress, winui.PBM_SETRANGE32, 0, 100)
	winui.BlueProgress(ui.progress)
	winui.Show(h)
	winui.Loop()
}
func ctrl(class, text string, style uint32, id int) winui.HWND {
	if class == "BUTTON" {
		style = (style &^ winui.BS_DEFPUSHBUTTON) | winui.BS_OWNERDRAW
	}
	return winui.CreateControl(ui.main, class, text, style, 0, id)
}
func (s *state) layout() {
	if s.main == 0 || s.heading == 0 {
		return
	}
	w, h := winui.Client(s.main)
	if w < 720 {
		w = 720
	}
	if h < 400 {
		h = 400
	}
	winui.Move(s.heading, 105, 17, w-155, 35)
	winui.Move(s.sub, 105, 57, w-120, 25)
	winui.Move(s.information, 34, 129, w-69, 50)
	winui.Move(s.source, 34, 189, w-69, 28)
	winui.Move(s.target, 34, 222, w-69, 48)
	winui.Move(s.progress, 34, 288, w-145, 18)
	winui.Move(s.pct, w-104, 282, 70, 28)
	winui.Move(s.choose, 34, 324, 188, 43)
	winui.Move(s.build, w-363, 324, 200, 43)
	winui.Move(s.closeBtn, w-152, 324, 119, 43)
	winui.Move(s.footer, 28, h-33, w-56, 22)
}
func (s *state) draw() {
	w, h := winui.Client(s.main)
	if w < 720 {
		w = 720
	}
	if h < 400 {
		h = 400
	}
	winui.PaintPanels(s.main, 95, []winui.Panel{{X: 16, Y: 104, W: w - 32, H: 278, Radius: 22, Fill: winui.RGB(255, 255, 255), Border: winui.RGB(209, 228, 250)}}, s.logo)
}
func loadIcon() uintptr {
	b, e := payload.ReadFile("payload/icon.ico")
	if e != nil {
		return 0
	}
	f, e := os.CreateTemp("", "pmc-builder-*.ico")
	if e != nil {
		return 0
	}
	name := f.Name()
	_, e = f.Write(b)
	f.Close()
	if e != nil {
		os.Remove(name)
		return 0
	}
	icon := winui.LoadIcon(name)
	os.Remove(name)
	return icon
}
func outputPath() string {
	home, e := os.UserHomeDir()
	if e != nil || home == "" {
		home = os.Getenv("USERPROFILE")
	}
	dir := filepath.Join(home, "Downloads")
	if _, e := os.Stat(dir); e != nil {
		dir = home
	}
	return filepath.Join(dir, "Perplex-Media-Converter-Setup-1.1.6-Offline.exe")
}

// First try a selected arbitrary FFmpeg folder, then convenient local folders.
// An older Perplex Media Converter installation is never required.
func findTools() (string, string, error) {
	exe, _ := os.Executable()
	base := filepath.Dir(exe)
	wd, _ := os.Getwd()
	dirs := []string{
		base,
		filepath.Join(base, "ffmpeg", "bin"),
		filepath.Join(base, "third_party", "ffmpeg", "bin"),
		filepath.Join(wd, "third_party", "ffmpeg", "bin"),
		filepath.Join(wd, "ffmpeg", "bin"),
		`C:\ffmpeg\bin`,
	}
	for _, dir := range dirs {
		a, b := filepath.Join(dir, "ffmpeg.exe"), filepath.Join(dir, "ffprobe.exe")
		if exists(a) && exists(b) {
			return a, b, nil
		}
	}
	a, ea := exec.LookPath("ffmpeg.exe")
	b, eb := exec.LookPath("ffprobe.exe")
	if ea == nil && eb == nil {
		return a, b, nil
	}
	return "", "", fmt.Errorf("FFmpeg was not found. Click 'Choose FFmpeg...' and select ffmpeg.exe in the verified 1.1.6 Developer Kit folder that also contains ffprobe.exe. You do NOT need Perplex Media Converter 1.1.3")
}
func (s *state) tools() (string, string, error) {
	if s.ffmpegPath != "" && s.ffprobePath != "" && exists(s.ffmpegPath) && exists(s.ffprobePath) {
		return s.ffmpegPath, s.ffprobePath, nil
	}
	return findTools()
}
func (s *state) previewSource() string {
	a, _, e := s.tools()
	if e != nil {
		return "Choose FFmpeg to select a folder (no previous installation needed)"
	}
	return a
}
func (s *state) chooseTools() {
	a, err := winui.Dialog(s.main, false, "", "Choose ffmpeg.exe from an extracted FFmpeg Windows build", "FFmpeg executable\x00ffmpeg.exe\x00Executable files\x00*.exe\x00")
	if err != nil {
		winui.Msg(s.main, "FFmpeg selection error", err.Error(), winui.MB_ICONWARNING)
		return
	}
	if a == "" {
		return
	}
	if !strings.EqualFold(filepath.Base(a), "ffmpeg.exe") {
		winui.Msg(s.main, "Invalid file", "Select the file named ffmpeg.exe.", winui.MB_ICONWARNING)
		return
	}
	b := filepath.Join(filepath.Dir(a), "ffprobe.exe")
	if !exists(a) || !exists(b) {
		winui.Msg(s.main, "Missing FFprobe", "Both ffmpeg.exe and ffprobe.exe must be in the same directory. Please select a complete Windows FFmpeg build.", winui.MB_ICONWARNING)
		return
	}
	s.ffmpegPath, s.ffprobePath = a, b
	winui.SetText(s.source, "FFmpeg source: "+a)
}
func exists(s string) bool { f, e := os.Stat(s); return e == nil && !f.IsDir() && f.Size() > 1000 }
func (s *state) post(text string, pct int) {
	select {
	case s.events <- event{text: text, pct: pct}:
		winui.Post(s.main, wmEvent, 0, 0)
	default:
	}
}
func (s *state) begin() {
	a, b, err := s.tools()
	if err != nil {
		winui.Msg(s.main, "Missing FFmpeg", err.Error(), winui.MB_OK|winui.MB_ICONWARNING)
		return
	}
	if exists(s.out) {
		if winui.Msg(s.main, "Replace existing setup?", "Replace the existing offline installer at the displayed output path?", winui.MB_YESNO|winui.MB_ICONQUESTION) != winui.IDYES {
			return
		}
		if err := os.Remove(s.out); err != nil {
			winui.Msg(s.main, title, err.Error(), winui.MB_ICONERROR)
			return
		}
	}
	stub, err := payload.ReadFile("payload/setup-stub.exe")
	if err != nil {
		winui.Msg(s.main, title, err.Error(), winui.MB_ICONERROR)
		return
	}
	s.busy = true
	winui.Enable(s.build, false)
	winui.Enable(s.choose, false)
	winui.SetText(s.closeBtn, "Cancel")
	winui.SetText(s.information, "Preparing a self-contained setup package (no internet required) …")
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		err := offlinebundle.Build(ctx, s.out, stub, a, b, s.post)
		s.events <- event{done: true, err: err}
		winui.Post(s.main, wmEvent, 0, 0)
	}()
}
func (s *state) receive() {
	for j := 0; j < 64; j++ {
		var e event
		select {
		case e = <-s.events:
		default:
			return
		}
		if e.done {
			s.busy = false
			s.cancel = nil
			winui.Enable(s.build, true)
			winui.Enable(s.choose, true)
			winui.SetText(s.closeBtn, "Close")
			if e.err != nil {
				winui.SetText(s.information, "Could not finish offline setup: "+e.err.Error())
				winui.Msg(s.main, "Packaging failed", e.err.Error(), winui.MB_OK|winui.MB_ICONERROR)
			} else {
				s.complete = true
				winui.Send(s.progress, winui.PBM_SETPOS, 100, 0)
				winui.SetText(s.pct, "100%")
				winui.SetText(s.information, "Offline installer created successfully. No installation downloads required.")
				winui.Msg(s.main, "Offline setup ready", "Offline installer created:\n"+s.out+"\n\nYou can now use this file to install the converter on another Windows PC without internet access.\n\nWhen publishing, offer Perplex-FFmpeg-Source-1.1.6.zip beside this installer with a visible free source link.", winui.MB_OK|winui.MB_ICONINFORMATION)
			}
		} else {
			winui.SetText(s.information, e.text)
			winui.SetText(s.pct, fmt.Sprintf("%d%%", e.pct))
			winui.Send(s.progress, winui.PBM_SETPOS, uintptr(e.pct), 0)
		}
	}
	if len(s.events) > 0 {
		winui.Post(s.main, wmEvent, 0, 0)
	}
}

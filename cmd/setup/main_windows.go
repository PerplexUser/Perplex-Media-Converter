//go:build windows

package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

const (
	title   = "Perplex Media Converter — Setup"
	wmEvent = winui.WM_APP + 33
)

type installerEvent struct {
	message string
	percent int
	done    bool
	err     error
}
type Installer struct {
	main                                                                                 winui.HWND
	title, subtitle, badge, info, detail, footer, progress, progressPct, install, cancel winui.HWND
	font, heading, logo                                                                  uintptr
	busy, complete                                                                       bool
	cancelJob                                                                            context.CancelFunc
	events                                                                               chan installerEvent
}

var inst *Installer

func proc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	i := inst
	if i != nil && winui.HWND(hwnd) == i.main && i.main != 0 {
		switch msg {
		case winui.WM_ERASEBKGND:
			return 1
		case winui.WM_PAINT:
			i.draw()
			return 0
		case winui.WM_SIZE:
			i.layout()
			return 0
		case winui.WM_CTLCOLORSTATIC:
			if winui.HWND(lp) == i.title {
				return winui.LabelColor(wp, winui.RGB(255, 255, 255), winui.RGB(12, 37, 78))
			}
			if winui.HWND(lp) == i.badge {
				return winui.LabelColor(wp, winui.RGB(212, 242, 255), winui.RGB(18, 75, 127))
			}
			if winui.HWND(lp) == i.subtitle {
				return winui.LabelColor(wp, winui.RGB(187, 223, 255), winui.RGB(12, 37, 78))
			}
			if winui.HWND(lp) == i.footer {
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
				if !i.busy {
					if i.complete {
						i.launch()
					} else {
						i.begin()
					}
				}
			case 1002:
				if i.busy {
					if i.cancelJob != nil {
						i.cancelJob()
					}
				} else {
					winui.Destroy(i.main)
				}
			}
			return 0
		case wmEvent:
			i.receive()
			return 0
		case winui.WM_CLOSE:
			if i.busy {
				if winui.Msg(i.main, "Cancel installation?", "Setup has not finished. Cancel and close?", winui.MB_YESNO|winui.MB_ICONQUESTION) != winui.IDYES {
					return 0
				}
				if i.cancelJob != nil {
					i.cancelJob()
				}
			}
			winui.Destroy(i.main)
			return 0
		case winui.WM_DESTROY:
			if i.cancelJob != nil {
				i.cancelJob()
			}
			winui.FreeIcon(i.logo)
			winui.PostQuit(0)
			return 0
		}
	}
	return winui.DefWndProc(winui.HWND(hwnd), msg, wp, lp)
}
func main() {
	runtime.LockOSThread()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--uninstall":
			uninstall()
			return
		case "--uninstall-now":
			if len(os.Args) > 2 {
				removeInstall(os.Args[2])
			}
			return
		}
	}
	winui.CommonControls()
	inst = &Installer{events: make(chan installerEvent, 128)}
	if err := winui.Register("PerplexMediaConverterSetupV2", syscall.NewCallback(proc)); err != nil {
		winui.Msg(0, title, err.Error(), winui.MB_ICONERROR)
		return
	}
	h, err := winui.CreateMain("PerplexMediaConverterSetupV2", title, 750, 430)
	if err != nil {
		winui.Msg(0, title, err.Error(), winui.MB_ICONERROR)
		return
	}
	inst.main = h
	inst.font = winui.Font(17, false)
	inst.heading = winui.Font(27, true)
	inst.logo = loadSetupIcon()
	winui.SetIconFromHandle(inst.main, inst.logo)
	inst.title = control("STATIC", "Perplex Media Converter", 0, 1010)
	inst.subtitle = control("STATIC", "WINDOWS 10 / 11   ·   64-BIT   ·   v1.1.6", 0, 1011)
	inst.badge = control("STATIC", "LOCAL & PRIVATE", 0, 1017)
	inst.info = control("STATIC", "Installed in your user profile  ·  No administrator access needed", 0, 1012)
	inst.detail = control("STATIC", "Complete offline installation: FFmpeg and FFprobe are included. No internet access required.", 0, 1013)
	inst.footer = control("STATIC", "© by www.perplex.click  ·  MIT © Michael Lappenbusch", 0, 1014)
	inst.progress = control("msctls_progress32", "", 0, 1015)
	inst.progressPct = control("STATIC", "0%", 0, 1016)
	inst.install = control("BUTTON", "Install now", winui.WS_TABSTOP|winui.BS_DEFPUSHBUTTON, 1001)
	inst.cancel = control("BUTTON", "Close", winui.WS_TABSTOP, 1002)
	winui.SetFont(inst.title, inst.heading)
	for _, h := range []winui.HWND{inst.subtitle, inst.badge, inst.info, inst.detail, inst.footer, inst.progressPct, inst.install, inst.cancel} {
		winui.SetFont(h, inst.font)
	}
	inst.layout()
	winui.Send(inst.progress, winui.PBM_SETRANGE32, 0, 100)
	winui.BlueProgress(inst.progress)
	winui.Show(h)
	winui.Loop()
}
func control(class, text string, style uint32, id int) winui.HWND {
	if class == "BUTTON" {
		style = (style &^ winui.BS_DEFPUSHBUTTON) | winui.BS_OWNERDRAW
	}
	return winui.CreateControl(inst.main, class, text, style, 0, id)
}
func loadSetupIcon() uintptr {
	data, err := payload.ReadFile("payload/icon.ico")
	if err != nil {
		return 0
	}
	f, err := os.CreateTemp("", "perplex-setup-icon-*.ico")
	if err != nil {
		return 0
	}
	name := f.Name()
	if _, err = f.Write(data); err != nil {
		f.Close()
		os.Remove(name)
		return 0
	}
	f.Close()
	icon := winui.LoadIcon(name)
	os.Remove(name)
	return icon
}
func (i *Installer) layout() {
	if i.main == 0 || i.title == 0 {
		return
	}
	width, height := winui.Client(i.main)
	if width < 660 {
		width = 660
	}
	if height < 380 {
		height = 380
	}
	winui.Move(i.title, 106, 17, width-325, 38)
	winui.Move(i.subtitle, 107, 55, width-325, 21)
	winui.Move(i.badge, width-177, 32, 150, 24)
	winui.Move(i.info, 36, 121, width-72, 26)
	winui.Move(i.detail, 36, 164, width-72, 50)
	winui.Move(i.progress, 36, 241, width-157, 16)
	winui.Move(i.progressPct, width-105, 234, 64, 28)
	winui.Move(i.install, width-350, 281, 175, 38)
	winui.Move(i.cancel, width-163, 281, 127, 38)
	winui.Move(i.footer, 26, height-33, width-52, 24)
}
func (i *Installer) draw() {
	width, height := winui.Client(i.main)
	if width < 660 {
		width = 660
	}
	if height < 380 {
		height = 380
	}
	panels := []winui.Panel{{X: 17, Y: 100, W: width - 34, H: 238, Radius: 22, Fill: winui.RGB(255, 255, 255), Border: winui.RGB(209, 228, 250)}}
	winui.PaintPanels(i.main, 85, panels, i.logo)
}
func installPath() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		home, _ := os.UserHomeDir()
		local = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(local, "Programs", "Perplex Media Converter")
}
func (i *Installer) emit(s string, pct int) {
	select {
	case i.events <- installerEvent{message: s, percent: pct}:
		winui.Post(i.main, wmEvent, 0, 0)
	default:
	}
}
func (i *Installer) result(err error) {
	i.events <- installerEvent{done: true, err: err}
	winui.Post(i.main, wmEvent, 0, 0)
}
func (i *Installer) begin() {
	i.busy = true
	winui.Enable(i.install, false)
	winui.SetText(i.cancel, "Cancel")
	winui.SetText(i.info, "Preparing application files …")
	winui.SetText(i.progressPct, "0%")
	winui.SetText(i.detail, installPath())
	ctx, cancel := context.WithCancel(context.Background())
	i.cancelJob = cancel
	go func() { i.result(i.installAll(ctx)) }()
}
func (i *Installer) receive() {
	for j := 0; j < 64; j++ {
		var ev installerEvent
		select {
		case ev = <-i.events:
		default:
			return
		}
		if ev.done {
			i.busy = false
			i.cancelJob = nil
			winui.SetText(i.cancel, "Close")
			winui.Enable(i.install, true)
			if ev.err != nil {
				winui.SetText(i.info, "Setup has not finished.")
				winui.SetText(i.detail, ev.err.Error())
				winui.SetText(i.install, "Try again")
				winui.Msg(i.main, "Setup incomplete", ev.err.Error()+"\n\nSome files may already have been copied. You can run Setup again.", winui.MB_OK|winui.MB_ICONWARNING)
			} else {
				i.complete = true
				winui.Send(i.progress, winui.PBM_SETPOS, 100, 0)
				winui.SetText(i.progressPct, "100%")
				winui.SetText(i.info, "Installation completed successfully.")
				winui.SetText(i.detail, "Open the app from the Start menu. All media is processed locally.")
				winui.SetText(i.install, "Launch app")
			}
		} else {
			winui.Send(i.progress, winui.PBM_SETPOS, uintptr(ev.percent), 0)
			winui.SetText(i.progressPct, fmt.Sprintf("%d%%", ev.percent))
			winui.SetText(i.info, ev.message)
		}
	}
	if len(i.events) > 0 {
		winui.Post(i.main, wmEvent, 0, 0)
	}
}
func (i *Installer) launch() {
	exe := filepath.Join(installPath(), "Perplex Media Converter.exe")
	if err := exec.Command(exe).Start(); err != nil {
		winui.Msg(i.main, title, err.Error(), winui.MB_ICONERROR)
	} else {
		winui.Destroy(i.main)
	}
}
func (i *Installer) installAll(ctx context.Context) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	i.emit("Checking bundled offline components …", 2)
	bundle, err := offlinebundle.Open(self)
	if err != nil {
		return fmt.Errorf("This is a setup stub, not a complete offline installer: %w. Please use the Offline Builder to create the full Setup file", err)
	}
	defer bundle.Close()
	dir := installPath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("Could not create installation folder: %w", err)
	}
	var files []string
	if err := fs.WalkDir(payload, "payload", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files = append(files, strings.TrimPrefix(path, "payload/"))
		}
		return nil
	}); err != nil {
		return err
	}
	for idx, name := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		contents, err := payload.ReadFile("payload/" + name)
		if err != nil {
			return err
		}
		i.emit("Installing application files and licenses …", 5+idx*14/len(files))
		out := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(out, contents, 0644); err != nil {
			return fmt.Errorf("Could not write %s (is the app still open?): %w", name, err)
		}
	}
	i.emit("Verifying and unpacking FFmpeg …", 20)
	if err := bundle.Extract(ctx, "ffmpeg.exe", filepath.Join(dir, "ffmpeg.exe")); err != nil {
		return err
	}
	i.emit("Verifying and unpacking FFprobe …", 65)
	if err := bundle.Extract(ctx, "ffprobe.exe", filepath.Join(dir, "ffprobe.exe")); err != nil {
		return err
	}
	i.emit("Installing the offline uninstaller …", 83)
	if err := bundle.Extract(ctx, "uninstall.exe", filepath.Join(dir, "uninstall.exe")); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	i.emit("Creating Start menu shortcut …", 93)
	if err := startMenuShortcut(dir); err != nil {
		return fmt.Errorf("Installed app, but could not create Start menu shortcut: %w", err)
	}
	registerUninstall(dir)
	i.emit("Offline installation complete. No downloads needed.", 100)
	return nil
}
func powershell(script string) error {
	file, err := os.CreateTemp("", "perplex-*.ps1")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	// PowerShell 5.1 interprets UTF-8 correctly with a BOM.
	if _, err := file.Write(append([]byte{0xef, 0xbb, 0xbf}, []byte(script)...)); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", file.Name())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PowerShell: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func startMenuShortcut(dir string) error {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return errors.New("Could not find APPDATA directory")
	}
	link := filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", "Perplex Media Converter.lnk")
	script := "$ErrorActionPreference='Stop'\n" +
		"$w=New-Object -ComObject WScript.Shell\n" +
		"$s=$w.CreateShortcut(" + psQuote(link) + ")\n" +
		"$s.TargetPath=" + psQuote(filepath.Join(dir, "Perplex Media Converter.exe")) + "\n" +
		"$s.WorkingDirectory=" + psQuote(dir) + "\n" +
		"$s.Description='Perplex Media Converter — © by www.perplex.click'\n" +
		"$s.IconLocation=" + psQuote(filepath.Join(dir, "icon.ico")) + "\n" +
		"$s.Save()\n"
	return powershell(script)
}
func reg(args ...string) {
	cmd := exec.Command("reg.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = cmd.Run()
}
func registerUninstall(dir string) {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\PerplexMediaConverter`
	set := func(name, value string) { reg("add", key, "/v", name, "/t", "REG_SZ", "/d", value, "/f") }
	set("DisplayName", "Perplex Media Converter")
	set("DisplayVersion", "1.1.6")
	set("Publisher", "Michael Lappenbusch")
	set("InstallLocation", dir)
	set("DisplayIcon", filepath.Join(dir, "icon.ico"))
	set("UninstallString", "\""+filepath.Join(dir, "uninstall.exe")+"\" --uninstall")
	reg("add", key, "/v", "NoModify", "/t", "REG_DWORD", "/d", "1", "/f")
}
func uninstall() {
	dir := installPath()
	if winui.Msg(0, "Uninstall Perplex Media Converter", "Remove Perplex Media Converter, its FFmpeg files, and Start menu shortcut? Your converted media files will not be deleted.", winui.MB_YESNO|winui.MB_ICONQUESTION) != winui.IDYES {
		return
	}
	src, _ := os.Executable()
	tempFile, err := os.CreateTemp("", "perplex-uninstaller-*.exe")
	if err != nil {
		winui.Msg(0, title, err.Error(), winui.MB_ICONERROR)
		return
	}
	name := tempFile.Name()
	tempFile.Close()
	input, err := os.Open(src)
	if err == nil {
		defer input.Close()
		output, e := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0700)
		if e == nil {
			_, err = io.Copy(output, input)
			if e = output.Close(); err == nil {
				err = e
			}
		} else {
			err = e
		}
	}
	if err != nil {
		winui.Msg(0, title, err.Error(), winui.MB_ICONERROR)
		return
	}
	cmd := exec.Command(name, "--uninstall-now", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		winui.Msg(0, title, err.Error(), winui.MB_ICONERROR)
	}
}
func removeInstall(dir string) {
	if !strings.EqualFold(filepath.Clean(dir), filepath.Clean(installPath())) {
		winui.Msg(0, "Uninstall", "Invalid installation folder.", winui.MB_OK|winui.MB_ICONERROR)
		return
	}
	// The uninstaller runs from a separate temporary exe; the installed folder
	// can be removed completely as long as the GUI is closed.
	if err := os.RemoveAll(dir); err != nil {
		winui.Msg(0, "Uninstall", "Could not fully remove the installation folder. Please close Perplex Media Converter and try uninstalling again.\n\n"+err.Error(), winui.MB_OK|winui.MB_ICONWARNING)
		return
	}
	appdata := os.Getenv("APPDATA")
	if appdata != "" {
		_ = os.Remove(filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", "Perplex Media Converter.lnk"))
	}
	reg("delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\PerplexMediaConverter`, "/f")
	winui.Msg(0, "Uninstall", "Perplex Media Converter has been uninstalled. Your media files were not changed.", winui.MB_OK|winui.MB_ICONINFORMATION)
}

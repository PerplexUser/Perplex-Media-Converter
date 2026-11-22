//go:build windows

package winui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"path/filepath"
	"perplex-media-converter/internal/filedialog"
	"syscall"
	"unsafe"
)

type HWND uintptr

const (
	WS_OVERLAPPED             = 0x00000000
	WS_CAPTION                = 0x00C00000
	WS_SYSMENU                = 0x00080000
	WS_THICKFRAME             = 0x00040000
	WS_MINIMIZEBOX            = 0x00020000
	WS_MAXIMIZEBOX            = 0x00010000
	WS_CHILD                  = 0x40000000
	WS_CLIPCHILDREN           = 0x02000000
	WS_CLIPSIBLINGS           = 0x04000000
	WS_VISIBLE                = 0x10000000
	WS_TABSTOP                = 0x00010000
	WS_BORDER                 = 0x00800000
	BS_OWNERDRAW              = 0x0000000B
	WS_VSCROLL                = 0x00200000
	WS_EX_CLIENTEDGE          = 0x00000200
	ES_AUTOHSCROLL            = 0x0080
	ES_MULTILINE              = 0x0004
	ES_READONLY               = 0x0800
	ES_AUTOVSCROLL            = 0x0040
	CBS_DROPDOWNLIST          = 0x0003
	BS_PUSHBUTTON             = 0x00000000
	BS_DEFPUSHBUTTON          = 0x00000001
	SS_LEFT                   = 0
	SS_NOTIFY                 = 0x0100
	SW_SHOW                   = 5
	SW_HIDE                   = 0
	SW_SHOWNORMAL             = 1
	WM_DESTROY                = 0x0002
	WM_PAINT                  = 0x000F
	WM_ERASEBKGND             = 0x0014
	WM_DRAWITEM               = 0x002B
	WM_CTLCOLORSTATIC         = 0x0138
	WM_CTLCOLOREDIT           = 0x0133
	WM_CLOSE                  = 0x0010
	WM_COMMAND                = 0x0111
	WM_SIZE                   = 0x0005
	WM_DROPFILES              = 0x0233
	WM_TIMER                  = 0x0113
	WM_SETFONT                = 0x0030
	WM_SETICON                = 0x0080
	WM_APP                    = 0x8000
	CB_ADDSTRING              = 0x0143
	CB_RESETCONTENT           = 0x014B
	CB_GETCURSEL              = 0x0147
	CB_SETCURSEL              = 0x014E
	CBN_SELCHANGE             = 1
	BN_CLICKED                = 0
	EM_SETSEL                 = 0x00B1
	EM_REPLACESEL             = 0x00C2
	PBM_SETRANGE32            = 0x0400 + 6
	PBM_SETPOS                = 0x0400 + 2
	PBM_SETBARCOLOR           = 0x0400 + 9
	PBM_SETBKCOLOR            = 0x0400 + 1
	MB_OK                     = 0x0000
	MB_ICONERROR              = 0x0010
	MB_ICONINFORMATION        = 0x0040
	MB_ICONWARNING            = 0x0030
	MB_YESNO                  = 0x0004
	MB_ICONQUESTION           = 0x0020
	IDYES                     = 6
	OFN_FILEMUSTEXIST         = 0x00001000
	OFN_PATHMUSTEXIST         = 0x00000800
	OFN_HIDEREADONLY          = 0x4
	OFN_OVERWRITEPROMPT       = 0x2
	OFN_EXPLORER              = 0x00080000
	IMAGE_ICON                = 1
	LR_LOADFROMFILE           = 0x00000010
	ICON_BIG                  = 1
	ICON_SMALL                = 0
	MOVEFILE_REPLACE_EXISTING = 1
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	comctl32             = syscall.NewLazyDLL("comctl32.dll")
	pGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	pRegisterClassEx     = user32.NewProc("RegisterClassExW")
	pCreateWindowEx      = user32.NewProc("CreateWindowExW")
	pDefWindowProc       = user32.NewProc("DefWindowProcW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pUpdateWindow        = user32.NewProc("UpdateWindow")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pGetMessage          = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessage     = user32.NewProc("DispatchMessageW")
	pSendMessage         = user32.NewProc("SendMessageW")
	pPostMessage         = user32.NewProc("PostMessageW")
	pSetWindowText       = user32.NewProc("SetWindowTextW")
	pGetWindowText       = user32.NewProc("GetWindowTextW")
	pGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	pMoveWindow          = user32.NewProc("MoveWindow")
	pEnableWindow        = user32.NewProc("EnableWindow")
	pMessageBox          = user32.NewProc("MessageBoxW")
	pLoadCursor          = user32.NewProc("LoadCursorW")
	pLoadImage           = user32.NewProc("LoadImageW")
	pCreateFont          = gdi32.NewProc("CreateFontW")
	pDeleteObject        = gdi32.NewProc("DeleteObject")
	pCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	pCreatePen           = gdi32.NewProc("CreatePen")
	pSelectObject        = gdi32.NewProc("SelectObject")
	pRoundRect           = gdi32.NewProc("RoundRect")
	pFillRect            = user32.NewProc("FillRect")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pSetBkMode           = gdi32.NewProc("SetBkMode")
	pSetTextColor        = gdi32.NewProc("SetTextColor")
	pSetBkColor          = gdi32.NewProc("SetBkColor")
	pDrawText            = user32.NewProc("DrawTextW")
	pGetStockObject      = gdi32.NewProc("GetStockObject")
	pRtlMoveMemory       = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlMoveMemory")
	pSetWindowTheme      = syscall.NewLazyDLL("uxtheme.dll").NewProc("SetWindowTheme")
	pDrawIconEx          = user32.NewProc("DrawIconEx")
	pGetClientRect       = user32.NewProc("GetClientRect")
	pSetTimer            = user32.NewProc("SetTimer")
	pKillTimer           = user32.NewProc("KillTimer")
	pGetOpenFileName     = comdlg32.NewProc("GetOpenFileNameW")
	pGetSaveFileName     = comdlg32.NewProc("GetSaveFileNameW")
	pShellExecute        = shell32.NewProc("ShellExecuteW")
	pDragAcceptFiles     = shell32.NewProc("DragAcceptFiles")
	pDragQueryFile       = shell32.NewProc("DragQueryFileW")
	pDragFinish          = shell32.NewProc("DragFinish")
	pInitCommonControls  = comctl32.NewProc("InitCommonControlsEx")
	pMoveFileEx          = kernel32.NewProc("MoveFileExW")
)

func Ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func Module() uintptr      { h, _, _ := pGetModuleHandle.Call(0); return h }
func DefWndProc(hwnd HWND, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := pDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wp, lp)
	return r
}
func PostQuit(code int) { pPostQuitMessage.Call(uintptr(code)) }
func Show(w HWND)       { pShowWindow.Call(uintptr(w), SW_SHOW); pUpdateWindow.Call(uintptr(w)) }
func Destroy(w HWND)    { pDestroyWindow.Call(uintptr(w)) }
func Send(w HWND, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := pSendMessage.Call(uintptr(w), uintptr(msg), wparam, lparam)
	return r
}
func Post(w HWND, msg uint32, wparam, lparam uintptr) {
	pPostMessage.Call(uintptr(w), uintptr(msg), wparam, lparam)
}
func SetText(w HWND, s string) { pSetWindowText.Call(uintptr(w), uintptr(unsafe.Pointer(Ptr(s)))) }
func GetText(w HWND) string {
	n, _, _ := pGetWindowTextLength.Call(uintptr(w))
	buf := make([]uint16, n+1)
	pGetWindowText.Call(uintptr(w), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func Move(w HWND, x, y, width, height int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	pMoveWindow.Call(uintptr(w), uintptr(x), uintptr(y), uintptr(width), uintptr(height), 1)
}
func ShowControl(w HWND, visible bool) {
	mode := uintptr(SW_HIDE)
	if visible {
		mode = SW_SHOW
	}
	pShowWindow.Call(uintptr(w), mode)
}
func Enable(w HWND, enable bool) {
	v := uintptr(0)
	if enable {
		v = 1
	}
	pEnableWindow.Call(uintptr(w), v)
}
func Msg(w HWND, caption, message string, flags uint32) int {
	r, _, _ := pMessageBox.Call(uintptr(w), uintptr(unsafe.Pointer(Ptr(message))), uintptr(unsafe.Pointer(Ptr(caption))), uintptr(flags))
	return int(r)
}
func Timer(w HWND, id, ms uintptr) { pSetTimer.Call(uintptr(w), id, ms, 0) }
func StopTimer(w HWND, id uintptr) { pKillTimer.Call(uintptr(w), id) }
func Client(w HWND) (int, int) {
	var r Rect
	pGetClientRect.Call(uintptr(w), uintptr(unsafe.Pointer(&r)))
	return int(r.Right - r.Left), int(r.Bottom - r.Top)
}
func AddCombo(w HWND, v string) { Send(w, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(Ptr(v)))) }
func ResetCombo(w HWND)         { Send(w, CB_RESETCONTENT, 0, 0) }
func SelectCombo(w HWND, i int) { Send(w, CB_SETCURSEL, uintptr(i), 0) }
func SelectedCombo(w HWND) int {
	v := Send(w, CB_GETCURSEL, 0, 0)
	if v == ^uintptr(0) {
		return -1
	}
	return int(v)
}
func Font(size int, bold bool) uintptr {
	weight := uintptr(400)
	if bold {
		weight = 700
	}
	h, _, _ := pCreateFont.Call(uintptr(size), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(Ptr("Segoe UI"))))
	return h
}
func SetFont(w HWND, font uintptr) { Send(w, WM_SETFONT, font, 1) }
func SetIconFromFile(w HWND, path string) {
	h := LoadIcon(path)
	if h != 0 {
		SetIconFromHandle(w, h)
	}
}
func CreateControl(parent HWND, class, text string, style, extra uint32, id int) HWND {
	h, _, _ := pCreateWindowEx.Call(uintptr(extra), uintptr(unsafe.Pointer(Ptr(class))), uintptr(unsafe.Pointer(Ptr(text))), uintptr(style|WS_CHILD|WS_VISIBLE), 0, 0, 1, 1, uintptr(parent), uintptr(id), Module(), 0)
	return HWND(h)
}

type WndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSmall  uintptr
}

func Register(class string, proc uintptr) error {
	cursor, _, _ := pLoadCursor.Call(0, 32512) // IDC_ARROW
	wc := WndClassEx{Size: uint32(unsafe.Sizeof(WndClassEx{})), WndProc: proc, Instance: Module(), Cursor: cursor, Background: 6, ClassName: Ptr(class)}
	r, _, err := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}
	return nil
}
func CreateMain(class, title string, width, height int) (HWND, error) {
	style := uint32(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_CLIPCHILDREN | WS_CLIPSIBLINGS)
	h, _, err := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(Ptr(class))), uintptr(unsafe.Pointer(Ptr(title))), uintptr(style), 80, 45, uintptr(width), uintptr(height), 0, 0, Module(), 0)
	if h == 0 {
		return 0, fmt.Errorf("CreateWindowEx: %w", err)
	}
	return HWND(h), nil
}

type Point struct{ X, Y int32 }
type WinMessage struct {
	Wnd     uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
	Private uint32
}
type Rect struct{ Left, Top, Right, Bottom int32 }

func Loop() int {
	var m WinMessage
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return int(m.WParam)
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}
func CommonControls() {
	x := struct {
		Size  uint32
		Flags uint32
	}{8, 0x00000020 | 0x00004000}
	pInitCommonControls.Call(uintptr(unsafe.Pointer(&x)))
}
func DragAccept(w HWND) { pDragAcceptFiles.Call(uintptr(w), 1) }
func DraggedPath(handle uintptr) string {
	defer pDragFinish.Call(handle)
	n, _, _ := pDragQueryFile.Call(handle, ^uintptr(0), 0, 0)
	if n < 1 {
		return ""
	}
	l, _, _ := pDragQueryFile.Call(handle, 0, 0, 0)
	buf := make([]uint16, l+1)
	pDragQueryFile.Call(handle, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func Open(path string) error {
	r, _, err := pShellExecute.Call(0, uintptr(unsafe.Pointer(Ptr("open"))), uintptr(unsafe.Pointer(Ptr(path))), 0, 0, SW_SHOWNORMAL)
	if r <= 32 {
		return fmt.Errorf("Could not open file: %w", err)
	}
	return nil
}
func ReplaceFile(temp, dest string) error {
	r, _, err := pMoveFileEx.Call(uintptr(unsafe.Pointer(Ptr(temp))), uintptr(unsafe.Pointer(Ptr(dest))), MOVEFILE_REPLACE_EXISTING)
	if r == 0 {
		return fmt.Errorf("Could not save output file: %w", err)
	}
	return nil
}

// Dialog runs the Windows Forms file picker in a separate, single-threaded-apartment
// Windows PowerShell process. The previous in-process common dialog could terminate
// the Go GUI on some Windows systems. A picker failure now returns an error while
// the converter itself remains open. PowerShell 5.1 is built into Windows 10/11.
func Dialog(_ HWND, save bool, initial, title, filter string) (string, error) {
	script, err := filedialog.Script(save, initial, title, filter)
	if err != nil {
		return "", err
	}
	encoded := filedialog.EncodeCommand(script)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encoded)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	// Never combine streams: Windows PowerShell may write a CLIXML progress
	// record on stderr even after a successful FILE: or CANCEL on stdout.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	execErr := cmd.Run()
	selected, parseErr := filedialog.ParseOutput(stdout.String())
	if parseErr == nil && execErr == nil {
		return selected, nil
	}
	if parseErr != nil && stdout.Len() != 0 {
		return "", parseErr
	}
	if execErr != nil {
		// CLIXML contains internal progress information, not a useful diagnosis;
		// report process failure without displaying raw serialized XML to users.
		return "", fmt.Errorf("Windows file picker failed to run (%v)", execErr)
	}
	return "", parseErr
}
func MediaFilter() string {
	return "Video and audio\x00*.mkv;*.mp4;*.avi;*.mov;*.webm;*.flac;*.wav;*.mp3;*.m4a\x00All files\x00*.*\x00"
}
func ExternalFilter() string {
	return "Audio files / media\x00*.aac;*.ac3;*.aif;*.aiff;*.alac;*.flac;*.m4a;*.mka;*.mp3;*.ogg;*.opus;*.wav;*.wma;*.mp4;*.mkv\x00All files\x00*.*\x00"
}
func OutputFilter(ext string) string {
	return stringsUpper(ext) + " file\x00*." + ext + "\x00All files\x00*.*\x00"
}
func stringsUpper(s string) string {
	var b = []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
func MainIconPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "icon.ico")
}

// All dashboard rendering uses built-in GDI. No WebView, external UI libraries,
// or dynamically downloaded application code are required.
func RGB(r, g, b byte) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

type PaintStruct struct {
	Hdc       uintptr
	Erase     int32
	Paint     Rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type Panel struct {
	X, Y, W, H, Radius int
	Fill, Border       uint32
}

func rounded(hdc uintptr, x, y, w, h, radius int, fill, border uint32) {
	if w <= 0 || h <= 0 {
		return
	}
	brush, _, _ := pCreateSolidBrush.Call(uintptr(fill))
	pen, _, _ := pCreatePen.Call(0, 1, uintptr(border))
	oldBrush, _, _ := pSelectObject.Call(hdc, brush)
	oldPen, _, _ := pSelectObject.Call(hdc, pen)
	pRoundRect.Call(hdc, uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h), uintptr(radius), uintptr(radius))
	pSelectObject.Call(hdc, oldBrush)
	pSelectObject.Call(hdc, oldPen)
	pDeleteObject.Call(brush)
	pDeleteObject.Call(pen)
}
func solid(hdc uintptr, x, y, w, h int, color uint32) {
	if w <= 0 || h <= 0 {
		return
	}
	brush, _, _ := pCreateSolidBrush.Call(uintptr(color))
	rect := Rect{int32(x), int32(y), int32(x + w), int32(y + h)}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), brush)
	pDeleteObject.Call(brush)
}

// PaintPanels paints the top band and all the rounded, lightly shadowed sections.
func PaintPanels(w HWND, headerHeight int, panels []Panel, logo uintptr) {
	var ps PaintStruct
	dc, _, _ := pBeginPaint.Call(uintptr(w), uintptr(unsafe.Pointer(&ps)))
	if dc == 0 {
		return
	}
	defer pEndPaint.Call(uintptr(w), uintptr(unsafe.Pointer(&ps)))
	width, height := Client(w)
	solid(dc, 0, 0, width, height, RGB(239, 245, 253))
	solid(dc, 0, 0, width, headerHeight, RGB(12, 37, 78))
	solid(dc, 0, headerHeight-4, width, 4, RGB(34, 153, 235))
	// An accent on the right of the title bar mirrors the turquoise in the icon.
	rounded(dc, width-198, 20, 174, 40, 24, RGB(18, 75, 127), RGB(27, 103, 167))
	for _, p := range panels {
		rounded(dc, p.X, p.Y+2, p.W, p.H, p.Radius, RGB(224, 234, 248), RGB(224, 234, 248))
		rounded(dc, p.X, p.Y, p.W, p.H, p.Radius, p.Fill, p.Border)
	}
	if logo != 0 {
		pDrawIconEx.Call(dc, 20, 11, logo, 62, 62, 0, 0, 3)
	}
}

var brushCache = struct {
	sync.Mutex
	brushes map[uint32]uintptr
}{brushes: map[uint32]uintptr{}}

// Opaque background brushes are essential for STATIC controls whose caption
// changes. Returning NULL_BRUSH left old letters on screen after SetWindowText.
func LabelColor(hdc uintptr, textColor, background uint32) uintptr {
	pSetBkMode.Call(hdc, 2) // OPAQUE
	pSetTextColor.Call(hdc, uintptr(textColor))
	pSetBkColor.Call(hdc, uintptr(background))
	brushCache.Lock()
	defer brushCache.Unlock()
	if brush := brushCache.brushes[background]; brush != 0 {
		return brush
	}
	brush, _, _ := pCreateSolidBrush.Call(uintptr(background))
	brushCache.brushes[background] = brush
	return brush
}

func StaticColor(hdc uintptr, textColor uint32) uintptr {
	return LabelColor(hdc, textColor, RGB(255, 255, 255))
}
func EditColor(hdc uintptr, textColor uint32) uintptr {
	return LabelColor(hdc, textColor, RGB(255, 255, 255))
}

type DrawItemStruct struct {
	CtlType, CtlID, ItemID, Action, State uint32
	HwndItem, Hdc                         uintptr
	Item                                  Rect
	ItemData                              uintptr
}

func DrawButton(lparam uintptr, bg, fg, border uint32) {
	if lparam == 0 {
		return
	}
	var value DrawItemStruct
	pRtlMoveMemory.Call(uintptr(unsafe.Pointer(&value)), lparam, unsafe.Sizeof(value))
	item := &value
	if item.Hdc == 0 {
		return
	}
	if item.State&4 != 0 { // disabled
		bg = RGB(233, 240, 249)
		fg = RGB(143, 159, 179)
		border = RGB(224, 231, 241)
	} else if item.State&1 != 0 { // mouse is down
		bg = RGB(21, 76, 159)
		fg = RGB(255, 255, 255)
		border = bg
	}
	w := int(item.Item.Right - item.Item.Left)
	h := int(item.Item.Bottom - item.Item.Top)
	rounded(item.Hdc, int(item.Item.Left)+1, int(item.Item.Top)+1, w-2, h-2, 13, bg, border)
	title := GetText(HWND(item.HwndItem))
	if title == "" {
		return
	}
	pSetBkMode.Call(item.Hdc, 1)
	pSetTextColor.Call(item.Hdc, uintptr(fg))
	// Use the control's assigned Segoe UI font so labels remain consistent.
	font := Send(HWND(item.HwndItem), 0x0031, 0, 0) // WM_GETFONT
	if font != 0 {
		old, _, _ := pSelectObject.Call(item.Hdc, font)
		defer pSelectObject.Call(item.Hdc, old)
	}
	r := item.Item
	r.Left += 8
	r.Right -= 8
	pDrawText.Call(item.Hdc, uintptr(unsafe.Pointer(Ptr(title))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), 0x0001|0x0004|0x0020|0x8000)
}
func LoadIcon(path string) uintptr {
	h, _, _ := pLoadImage.Call(0, uintptr(unsafe.Pointer(Ptr(path))), IMAGE_ICON, 64, 64, LR_LOADFROMFILE)
	return h
}
func SetIconFromHandle(w HWND, icon uintptr) {
	if icon == 0 {
		return
	}
	Send(w, WM_SETICON, ICON_BIG, icon)
	Send(w, WM_SETICON, ICON_SMALL, icon)
}
func FreeIcon(h uintptr) {
	if h != 0 {
		pDestroyIcon := user32.NewProc("DestroyIcon")
		pDestroyIcon.Call(h)
	}
}

// Set the progress meter to the application's blue palette rather than green.
func BlueProgress(w HWND) {
	pSetWindowTheme.Call(uintptr(w), uintptr(unsafe.Pointer(Ptr(""))), uintptr(unsafe.Pointer(Ptr(""))))
	Send(w, PBM_SETBARCOLOR, 0, uintptr(RGB(30, 111, 218)))
	Send(w, PBM_SETBKCOLOR, 0, uintptr(RGB(224, 239, 255)))
}

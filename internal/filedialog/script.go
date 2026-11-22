package filedialog

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// EncodeCommand returns the UTF-16LE/base64 representation expected by
// powershell.exe -EncodedCommand. It contains no executable user-supplied text.
func EncodeCommand(script string) string {
	words := utf16.Encode([]rune(script))
	data := make([]byte, len(words)*2)
	for i, w := range words {
		binary.LittleEndian.PutUint16(data[i*2:], w)
	}
	return base64.StdEncoding.EncodeToString(data)
}
func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// Script creates a Windows-native open/save dialog in a dedicated STA process.
// The file name is returned as base64 UTF-8 to keep unicode paths safe across
// PowerShell 5.1 console encodings. User input is always base64 data, not code.
func Script(save bool, initial, title, filter string) (string, error) {
	fields := strings.Split(strings.TrimRight(filter, "\x00"), "\x00")
	if len(fields) < 2 || len(fields)%2 != 0 {
		return "", errors.New("Invalid file dialog filter")
	}
	for _, f := range fields {
		if f == "" || strings.Contains(f, "|") {
			return "", errors.New("Invalid file dialog filter entry")
		}
	}
	kind := "OpenFileDialog"
	specifics := "$dialog.CheckFileExists = $true\n"
	if save {
		kind = "SaveFileDialog"
		specifics = "$dialog.OverwritePrompt = $true\n$dialog.AddExtension = $true\n"
	}
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'
try {
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -AssemblyName System.Drawing
    [System.Windows.Forms.Application]::EnableVisualStyles()
    function D([string]$s) { [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($s)) }
    $dialog = New-Object System.Windows.Forms.%s
    $dialog.Title = D '%s'
    $dialog.Filter = D '%s'
    $dialog.FileName = D '%s'
    $dialog.CheckPathExists = $true
    $dialog.RestoreDirectory = $true
    %s
    $owner = New-Object System.Windows.Forms.Form
    $owner.FormBorderStyle = 'None'
    $owner.ShowInTaskbar = $false
    $owner.StartPosition = 'Manual'
    $owner.Location = [System.Drawing.Point]::new(-31000, -31000)
    $owner.Size = [System.Drawing.Size]::new(1, 1)
    $owner.Opacity = 0
    $owner.TopMost = $true
    try {
        $owner.Show()
        $answer = $dialog.ShowDialog($owner)
        if ($answer -eq [System.Windows.Forms.DialogResult]::OK) {
            [Console]::Out.WriteLine('FILE:' + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($dialog.FileName)))
        } else {
            [Console]::Out.WriteLine('CANCEL')
        }
    } finally {
        $owner.Close()
        $owner.Dispose()
        $dialog.Dispose()
    }
} catch {
    [Console]::Out.WriteLine('ERROR:' + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($_.Exception.Message)))
    exit 1
}
`, kind, b64(title), b64(strings.Join(fields, "|")), b64(initial), specifics)
	return script, nil
}

// ParseOutput reads only the file-picker protocol from standard output.
// Windows PowerShell 5.1 can emit non-fatal progress records formatted as
// CLIXML. Those records must never be mistaken for a selected file path.
// Keeping stdout separate from stderr prevents them contaminating the result.
func ParseOutput(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "\ufeff"))
		switch {
		case strings.HasPrefix(line, "FILE:"):
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(line, "FILE:")))
			if err != nil || len(raw) == 0 || !utf8.Valid(raw) {
				return "", errors.New("File picker returned an invalid file path")
			}
			return string(raw), nil
		case line == "CANCEL":
			return "", nil
		case strings.HasPrefix(line, "ERROR:"):
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(line, "ERROR:")))
			if err != nil || !utf8.Valid(raw) {
				return "", errors.New("Windows file picker reported an unreadable error")
			}
			return "", fmt.Errorf("Windows file picker: %s", string(raw))
		}
	}
	return "", errors.New("Windows file picker did not return a selection or cancellation status")
}

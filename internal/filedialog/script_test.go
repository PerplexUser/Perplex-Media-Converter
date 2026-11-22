package filedialog

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestPowerShellScriptSafeUnicodePaths(t *testing.T) {
	path := `C:\Users\Michaël O'Brien\Videos\🐱 - clip.mp4`
	script, err := Script(false, path, `Browse 'files'`, "Media\x00*.mp4;*.mkv\x00All files\x00*.*\x00")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, path) {
		t.Fatal("unescaped user data in script")
	}
	for _, want := range []string{"OpenFileDialog", "ShowDialog($owner)", "FILE:", "CANCEL", base64.StdEncoding.EncodeToString([]byte(path)), "Media|*.mp4;*.mkv|All files|*.*"} {
		if want == "Media|*.mp4;*.mkv|All files|*.*" {
			want = base64.StdEncoding.EncodeToString([]byte(want))
		}
		if !strings.Contains(script, want) {
			t.Fatalf("missing %q", want)
		}
	}
	cmd, err := base64.StdEncoding.DecodeString(EncodeCommand(script))
	if err != nil {
		t.Fatal(err)
	}
	words := make([]uint16, len(cmd)/2)
	for i := range words {
		words[i] = binary.LittleEndian.Uint16(cmd[2*i:])
	}
	if string(utf16.Decode(words)) != script {
		t.Fatal("powershell encoding roundtrip failed")
	}
}
func TestSaveDialogAndInvalidFilter(t *testing.T) {
	script, err := Script(true, "test.mp4", "Save", "MP4\x00*.mp4\x00")
	if err != nil || !strings.Contains(script, "SaveFileDialog") || !strings.Contains(script, "OverwritePrompt") {
		t.Fatalf("save dialog: %v", err)
	}
	_, err = Script(false, "", "", "broken\x00")
	if err == nil {
		t.Fatal("wanted filter validation error")
	}
}

func TestParseOutputIgnoresPowerShellCLIXML(t *testing.T) {
	path := `C:\Users\Michaël O'Brien\Videos\🌟 Urlaub.mp4`
	token := base64.StdEncoding.EncodeToString([]byte(path))
	// Reproduces the screenshot: CLIXML banner precedes the FILE: protocol,
	// and PowerShell appends an XML progress record after it.
	output := "#< CLIXML\r\nFILE:" + token + "\r\n<Objs Version=\"1.1.0.1\"><Obj S=\"progress\">Completed</Obj></Objs>\r\n"
	got, err := ParseOutput(output)
	if err != nil || got != path {
		t.Fatalf("parse CLIXML-wrapped response: got %q, err %v", got, err)
	}
}

func TestParseOutputProtocol(t *testing.T) {
	tests := []struct{ value, path, errContains string }{
		{"CANCEL\r\n", "", ""},
		{"#< CLIXML\nCANCEL\n<Objs/>\n", "", ""},
		{"ERROR:" + base64.StdEncoding.EncodeToString([]byte("Could not open dialog")) + "\r\n", "", "Could not open dialog"},
		{"#< CLIXML\n<Objs/>\n", "", "did not return"},
		{"FILE:not valid base64\n", "", "invalid file path"},
		{"FILE:" + base64.StdEncoding.EncodeToString([]byte("D:/video.mkv")) + "\n", "D:/video.mkv", ""},
	}
	for _, tt := range tests {
		got, err := ParseOutput(tt.value)
		if got != tt.path {
			t.Errorf("%q: got path %q want %q", tt.value, got, tt.path)
		}
		if tt.errContains == "" && err != nil {
			t.Errorf("%q: unexpected err %v", tt.value, err)
		}
		if tt.errContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errContains)) {
			t.Errorf("%q: expected err %q, got %v", tt.value, tt.errContains, err)
		}
	}
}

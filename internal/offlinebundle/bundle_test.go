package offlinebundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakePE(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	data := append([]byte("MZ"), bytes.Repeat([]byte(name), 500)...)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func approvalForTest(t *testing.T, ff, fp string) releaseInfo {
	t.Helper()
	info := func(path string) FileInfo {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return FileInfo{Size: int64(len(b)), SHA256: fmt.Sprintf("%x", sha256.Sum256(b))}
	}
	return releaseInfo{FFmpeg: info(ff), FFprobe: info(fp), Source: SourceInfo{Archive: "test-source.zip", SHA256: strings.Repeat("a", 64)}}
}
func TestBuildOpenExtractOffline(t *testing.T) {
	d := t.TempDir()
	stub := append([]byte("MZ"), bytes.Repeat([]byte("setup"), 600)...)
	ff := fakePE(t, d, "ffmpeg-original.exe")
	fp := fakePE(t, d, "ffprobe-original.exe")
	setup := filepath.Join(d, "offline.exe")
	approved := approvalForTest(t, ff, fp)
	if e := build(context.Background(), setup, stub, ff, fp, approved, nil); e != nil {
		t.Fatal(e)
	}
	pkg, e := Open(setup)
	if e != nil {
		t.Fatal(e)
	}
	defer pkg.Close()
	if pkg.Manifest.Source != approved.Source {
		t.Fatal("source reference was lost")
	}
	if pkg.Manifest.FFmpeg.Size <= 1000 {
		t.Fatalf("bad ffmpeg manifest: %+v", pkg.Manifest)
	}
	dest := filepath.Join(d, "extracted-ffmpeg.exe")
	if e := pkg.Extract(context.Background(), "ffmpeg.exe", dest); e != nil {
		t.Fatal(e)
	}
	a, _ := os.ReadFile(ff)
	b, _ := os.ReadFile(dest)
	if !bytes.Equal(a, b) {
		t.Fatal("extracted executable differs from original")
	}
	if e := pkg.Extract(context.Background(), "uninstall.exe", filepath.Join(d, "uninstall.exe")); e != nil {
		t.Fatal(e)
	}
	if e := build(context.Background(), setup, stub, ff, fp, approved, nil); e == nil {
		t.Fatal("must not overwrite installer")
	}
}
func TestNoNetworkAndCancellation(t *testing.T) {
	d := t.TempDir()
	stub := append([]byte("MZ"), bytes.Repeat([]byte("stub"), 600)...)
	ff := fakePE(t, d, "a.exe")
	fp := fakePE(t, d, "b.exe")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := filepath.Join(d, "not-created.exe")
	if err := build(ctx, out, stub, ff, fp, approvalForTest(t, ff, fp), nil); err == nil {
		t.Fatal("canceled build must fail")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("canceled output should not exist")
	}
	if _, err := Open(ff); err == nil {
		t.Fatal("non-package must be rejected")
	}
}

func TestRejectUnmatchedToolWithoutPublishingInstaller(t *testing.T) {
	d := t.TempDir()
	stub := append([]byte("MZ"), bytes.Repeat([]byte("stub"), 600)...)
	ff, fp := fakePE(t, d, "ffmpeg.exe"), fakePE(t, d, "ffprobe.exe")
	approved := approvalForTest(t, ff, fp)
	f, err := os.OpenFile(fp, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("changed after source identification")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(d, "must-not-publish.exe")
	if err := build(context.Background(), out, stub, ff, fp, approved, nil); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected source mismatch, got %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("unapproved output exists")
	}
	leftovers, _ := filepath.Glob(filepath.Join(d, ".pmc-offline-*.exe"))
	if len(leftovers) != 0 {
		t.Fatal("failed build left temporary executables")
	}
	if err := Build(context.Background(), out, stub, ff, fp, nil); err == nil {
		t.Fatal("public builder accepted undocumented binaries")
	}
}

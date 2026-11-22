// Package offlinebundle creates and verifies self-contained Windows installers.
// The package is appended as a ZIP overlay to a Windows PE setup stub.
package offlinebundle

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const FormatVersion = 1
const MaxToolBytes int64 = 350 << 20

type FileInfo struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Format      int        `json:"format"`
	AppVersion  string     `json:"app_version"`
	PreparedUTC string     `json:"prepared_utc"`
	FFmpeg      FileInfo   `json:"ffmpeg"`
	FFprobe     FileInfo   `json:"ffprobe"`
	Notice      string     `json:"notice"`
	Source      SourceInfo `json:"corresponding_source"`
}

// SourceInfo identifies the source archive that must accompany this release.
type SourceInfo struct {
	Archive string `json:"archive"`
	SHA256  string `json:"sha256"`
}

type releaseInfo struct {
	FFmpeg  FileInfo   `json:"ffmpeg"`
	FFprobe FileInfo   `json:"ffprobe"`
	Source  SourceInfo `json:"corresponding_source"`
}

//go:embed approved-release.json
var approvedReleaseJSON []byte

type Package struct {
	reader   *zip.ReadCloser
	files    map[string]*zip.File
	Manifest Manifest
}

func peFile(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	header := make([]byte, 2)
	if _, err = io.ReadFull(f, header); err != nil {
		return 0, err
	}
	if string(header) != "MZ" {
		return 0, fmt.Errorf("%s is not a Windows executable (missing MZ header)", path)
	}
	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if stat.Size() < 1000 || stat.Size() > MaxToolBytes {
		return 0, fmt.Errorf("%s has an unexpected size", path)
	}
	return stat.Size(), nil
}

// Build assembles one portable setup exe. This is done ONCE on the packager's PC;
// the installer produced by Build does not require the internet.
func Build(ctx context.Context, destination string, stub []byte, ffmpegPath, ffprobePath string, update func(string, int)) (err error) {
	var approved releaseInfo
	if err := json.Unmarshal(approvedReleaseJSON, &approved); err != nil {
		return fmt.Errorf("Read approved release: %w", err)
	}
	return build(ctx, destination, stub, ffmpegPath, ffprobePath, approved, update)
}

func build(ctx context.Context, destination string, stub []byte, ffmpegPath, ffprobePath string, approved releaseInfo, update func(string, int)) (err error) {
	if approved.Source.Archive == "" || filepath.Base(approved.Source.Archive) != approved.Source.Archive || len(approved.Source.SHA256) != 64 {
		return errors.New("Missing corresponding-source release metadata")
	}
	if len(stub) < 1000 || string(stub[:2]) != "MZ" {
		return errors.New("Invalid Windows setup stub")
	}
	paths := []struct{ name, path string }{{"ffmpeg.exe", ffmpegPath}, {"ffprobe.exe", ffprobePath}}
	for _, p := range paths {
		if _, e := peFile(p.path); e != nil {
			return e
		}
	}
	if st, e := os.Stat(destination); e == nil && !st.IsDir() {
		return fmt.Errorf("Output file already exists: %s", destination)
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(destination), ".pmc-offline-*.exe")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer func() {
		if err != nil {
			os.Remove(temp)
		}
	}()
	if _, err = f.Write(stub); err != nil {
		f.Close()
		return err
	}
	zw := zip.NewWriter(f)
	manifest := Manifest{Format: FormatVersion, AppVersion: "1.1.6", PreparedUTC: time.Now().UTC().Format(time.RFC3339), Notice: "FFmpeg/FFprobe are GPLv3 binaries independently licensed from Perplex Media Converter (MIT). Redistributors must provide corresponding GPL sources and build information."}
	manifest.Source = approved.Source
	for j, p := range paths {
		if err = ctx.Err(); err != nil {
			break
		}
		if update != nil {
			update("Packaging "+p.name+" …", 8+j*42)
		}
		var dst io.Writer
		dst, err = zw.Create(p.name)
		if err != nil {
			break
		}
		var in *os.File
		in, err = os.Open(p.path)
		if err != nil {
			break
		}
		h := sha256.New()
		var n int64
		n, err = io.Copy(dst, io.TeeReader(io.LimitReader(in, MaxToolBytes+1), h))
		in.Close()
		if err != nil {
			break
		}
		if n > MaxToolBytes {
			err = errors.New("An FFmpeg executable exceeds the maximum size")
			break
		}
		info := FileInfo{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}
		expected := approved.FFmpeg
		if p.name == "ffprobe.exe" {
			expected = approved.FFprobe
		}
		if info != expected {
			err = fmt.Errorf("%s does not match the documented 1.1.6 source package. Use the supplied 1.1.6 tools; another FFmpeg build needs its own verified source package and release metadata", p.name)
			break
		}
		if p.name == "ffmpeg.exe" {
			manifest.FFmpeg = info
		} else {
			manifest.FFprobe = info
		}
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err == nil {
		if update != nil {
			update("Adding uninstall support …", 92)
		}
		var w io.Writer
		w, err = zw.Create("uninstall.exe")
		if err == nil {
			_, err = w.Write(stub)
		}
	}
	if err == nil {
		var w io.Writer
		w, err = zw.Create("bundle-manifest.json")
		if err == nil {
			var b []byte
			b, err = json.MarshalIndent(manifest, "", "  ")
			if err == nil {
				_, err = w.Write(b)
			}
		}
	}
	closeErr := zw.Close()
	if err == nil {
		err = closeErr
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	// Fully parse what was written before publishing it under the final file name.
	var pkg *Package
	pkg, err = Open(temp)
	if err != nil {
		return err
	}
	pkg.Close()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if update != nil {
		update("Finalizing offline installer …", 98)
	}
	err = os.Rename(temp, destination)
	return err
}

func Open(setupPath string) (*Package, error) {
	reader, err := zip.OpenReader(setupPath)
	if err != nil {
		return nil, fmt.Errorf("This setup has no offline FFmpeg package: %w", err)
	}
	p := &Package{reader: reader, files: make(map[string]*zip.File)}
	for _, z := range reader.File {
		if z.Name == "ffmpeg.exe" || z.Name == "ffprobe.exe" || z.Name == "uninstall.exe" || z.Name == "bundle-manifest.json" {
			if _, exists := p.files[z.Name]; exists {
				p.Close()
				return nil, fmt.Errorf("Duplicate package entry %s", z.Name)
			}
			p.files[z.Name] = z
		}
	}
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe", "uninstall.exe", "bundle-manifest.json"} {
		if p.files[name] == nil {
			p.Close()
			return nil, fmt.Errorf("Offline installer is missing %s", name)
		}
	}
	file, err := p.files["bundle-manifest.json"].Open()
	if err != nil {
		p.Close()
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(file, 16384))
	file.Close()
	if err != nil {
		p.Close()
		return nil, err
	}
	if err = json.Unmarshal(b, &p.Manifest); err != nil {
		p.Close()
		return nil, err
	}
	if p.Manifest.Format != FormatVersion || p.Manifest.AppVersion != "1.1.6" {
		p.Close()
		return nil, errors.New("Unsupported offline bundle version")
	}
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
		var info FileInfo
		if name == "ffmpeg.exe" {
			info = p.Manifest.FFmpeg
		} else {
			info = p.Manifest.FFprobe
		}
		if info.Size < 1000 || info.Size > MaxToolBytes || int64(p.files[name].UncompressedSize64) != info.Size || len(info.SHA256) != 64 {
			p.Close()
			return nil, fmt.Errorf("Invalid manifest for %s", name)
		}
		if _, e := hex.DecodeString(info.SHA256); e != nil {
			p.Close()
			return nil, fmt.Errorf("Invalid SHA-256 for %s", name)
		}
	}
	return p, nil
}
func (p *Package) Close() error { return p.reader.Close() }

// Extract copies a bundled executable into dest using a temporary file and
// verifies SHA-256 before making it available for the next conversion.
func (p *Package) Extract(ctx context.Context, name, dest string) error {
	z := p.files[name]
	if z == nil {
		return fmt.Errorf("Missing package file %s", name)
	}
	in, err := z.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	f, err := os.CreateTemp(filepath.Dir(dest), ".pmc-unpack-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	h := sha256.New()
	reader := &cancelReader{ctx: ctx, r: in}
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(reader, MaxToolBytes+1))
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	if name == "ffmpeg.exe" || name == "ffprobe.exe" {
		var info FileInfo
		if name == "ffmpeg.exe" {
			info = p.Manifest.FFmpeg
		} else {
			info = p.Manifest.FFprobe
		}
		if n != info.Size || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), info.SHA256) {
			return fmt.Errorf("Package integrity check failed for %s", name)
		}
	} else if n < 1000 || n > MaxToolBytes {
		return fmt.Errorf("Invalid bundled %s", name)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// Windows does not allow replacing an exe currently in use.
	if err = os.Rename(tmp, dest); err != nil {
		if err = os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Cannot replace %s: %w", dest, err)
		}
		return os.Rename(tmp, dest)
	}
	return nil
}

type cancelReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *cancelReader) Read(p []byte) (int, error) {
	if e := c.ctx.Err(); e != nil {
		return 0, e
	}
	return c.r.Read(p)
}

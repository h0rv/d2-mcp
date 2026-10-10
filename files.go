package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/h0rv/d2-mcp/d2"
)

// sourceFS snapshots each admitted file once per request. D2's validation and
// layout passes see identical imports, and never fall back to os.Open.
type sourceFS struct {
	root      *os.Root
	files     map[string][]byte
	remaining int
}

func (s *sourceFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || filepath.Ext(name) != ".d2" {
		return nil, fmt.Errorf("only relative .d2 paths inside --input-dir are allowed")
	}
	if data, ok := s.files[name]; ok {
		return memoryFile(name, data), nil
	}
	if s.root == nil {
		return nil, errors.New("file input and imports require --input-dir")
	}
	if len(s.files) >= 32 {
		return nil, errors.New("source exceeds 32 files")
	}
	info, err := s.root.Stat(name)
	if err != nil {
		return nil, errors.New("source file is unavailable inside --input-dir")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("source must be a regular file")
	}
	f, err := s.root.Open(name)
	if err != nil {
		return nil, errors.New("source file is unavailable inside --input-dir")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("source must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(s.remaining)+1))
	if err != nil {
		return nil, errors.New("failed to read source file")
	}
	if len(data) > s.remaining {
		return nil, errors.New("combined source exceeds 256 KiB")
	}
	s.remaining -= len(data)
	s.files[name] = data
	return memoryFile(name, data), nil
}

type sourceFile struct {
	*bytes.Reader
	name string
	size int64
}

func memoryFile(name string, data []byte) *sourceFile {
	return &sourceFile{bytes.NewReader(data), name, int64(len(data))}
}
func (f *sourceFile) Close() error               { return nil }
func (f *sourceFile) Stat() (fs.FileInfo, error) { return sourceInfo{f.name, f.size}, nil }

type sourceInfo struct {
	name string
	size int64
}

func (i sourceInfo) Name() string       { return path.Base(i.name) }
func (i sourceInfo) Size() int64        { return i.size }
func (i sourceInfo) Mode() fs.FileMode  { return 0444 }
func (i sourceInfo) ModTime() time.Time { return time.Time{} }
func (i sourceInfo) IsDir() bool        { return false }
func (i sourceInfo) Sys() any           { return nil }

func (a *application) source(args toolArgs) (d2.Source, error) {
	if (args.Code == nil) == (args.FilePath == nil) {
		return d2.Source{}, errors.New("provide exactly one of code or file_path")
	}
	snapshot := &sourceFS{root: a.input, files: make(map[string][]byte), remaining: d2.MaxSourceBytes}
	name := "input.d2"
	var code string
	if args.Code != nil {
		code = *args.Code
		if len(code) > d2.MaxSourceBytes {
			return d2.Source{}, errors.New("source exceeds 256 KiB")
		}
		snapshot.remaining -= len(code)
		snapshot.files[name] = []byte(code)
	} else {
		name = *args.FilePath
		f, err := snapshot.Open(name)
		if err != nil {
			return d2.Source{}, err
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return d2.Source{}, err
		}
		code = string(data)
	}
	if len(bytes.TrimSpace([]byte(code))) == 0 {
		return d2.Source{}, errors.New("D2 source must not be empty")
	}
	return d2.Source{Code: code, Path: name, FS: snapshot}, nil
}

func (a *application) writeArtifact(name, format string, data []byte) (string, error) {
	if a.output == nil {
		return "", errors.New("artifact writes require --output-dir")
	}
	ext := "." + format
	if format == "ascii" {
		ext = ".txt"
	}
	if name == "" {
		name = "diagram-" + rand.Text() + ext
	}
	if !fs.ValidPath(name) || filepath.Base(name) != name || len(name) > 200 || filepath.Ext(name) != ext {
		return "", fmt.Errorf("output_name must be a filename ending in %s, with no directories", ext)
	}
	// O_EXCL also refuses existing symlinks; inputs and existing artifacts are
	// never overwritten. A failed write removes only the file this call created.
	f, err := a.output.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", errors.New("cannot create artifact; choose a new output_name")
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		_ = a.output.Remove(name)
		return "", errors.New("failed to write artifact")
	}
	return filepath.Join(a.outputPath, name), nil
}

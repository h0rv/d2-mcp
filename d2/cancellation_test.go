package d2

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"testing"
	"testing/fstest"
)

// A normal read succeeds but the request is canceled at the read/parse or
// read/decode boundary. Invalid content must not hide the cancellation.
type cancelReadFS struct {
	fs.FS
	cancel context.CancelFunc
}

func (s cancelReadFS) Open(name string) (fs.File, error) {
	f, err := s.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return cancelReadFile{File: f, cancel: s.cancel}, nil
}

type cancelReadFile struct {
	fs.File
	cancel context.CancelFunc
}

func (f cancelReadFile) Read(data []byte) (int, error) {
	n, err := f.File.Read(data)
	f.cancel()
	return n, err
}

func TestPrepareStopsAfterCanceledImportRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	files := cancelReadFS{FS: fstest.MapFS{"library.d2": {Data: []byte("invalid: {")}}, cancel: cancel}
	_, err := Prepare(ctx, Source{Code: "...@library", Path: "input.d2", FS: files}, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("prepare error = %v", err)
	}
}

func TestAssetCancellationBeforeDecodeAndCacheHit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	files := cancelReadFS{FS: fstest.MapFS{"icon.png": {Data: []byte("not an image")}}, cancel: cancel}
	loader := newAssetLoader(ctx, files, false)
	icon := &url.URL{Path: "icon.png"}
	if _, err := loader.load(icon); !errors.Is(err, context.Canceled) {
		t.Fatalf("asset error = %v", err)
	}
	loader.cache[icon.String()] = &url.URL{Scheme: "data", Opaque: "image/png;base64,unused"}
	if _, err := loader.load(icon); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached asset error = %v", err)
	}
}

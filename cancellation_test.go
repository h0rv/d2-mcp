package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/h0rv/d2-mcp/document"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCancelledRequestDoesNotBecomeBusy(t *testing.T) {
	a := testApp(t, func(c *config) { c.Store = &testStore{} })
	a.slots <- struct{}{}
	a.slots <- struct{}{}
	defer func() { <-a.slots; <-a.slots }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, store := range []bool{false, true} {
		request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"code":"a"}`)}}
		var result *mcp.CallToolResult
		var err error
		if store {
			request.Params.Arguments = json.RawMessage(`{"document_id":"diagram","expected_revision":"","code":"a"}`)
			result, err = a.StoreD2Handler(ctx, request)
		} else {
			result, err = a.execute(ctx, request, true)
		}
		if err != nil {
			t.Fatal(err)
		}
		requireError(t, result, "cancelled")
	}
}

type uninterruptibleGetStore struct {
	entered, release chan struct{}
}

func (s *uninterruptibleGetStore) Get(context.Context, string) (document.Document, error) {
	close(s.entered)
	<-s.release
	return document.Document{ID: "diagram", Revision: "different", Code: "a"}, nil
}
func (*uninterruptibleGetStore) Put(context.Context, string, string, string) (document.Document, error) {
	panic("unexpected write")
}

func TestCancellationRetainsSlotUntilWorkExits(t *testing.T) {
	store := &uninterruptibleGetStore{entered: make(chan struct{}), release: make(chan struct{})}
	a := testApp(t, func(c *config) { c.Store = store })
	a.slots <- struct{}{}
	defer func() { <-a.slots }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan *mcp.CallToolResult, 1)
	go func() {
		r, _ := a.execute(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"document_id":"diagram","revision":"1"}`)}}, true)
		result <- r
	}()
	<-store.entered
	cancel()
	// This intentionally uncooperative backend demonstrates that cancellation
	// must not release a slot while the underlying operation still occupies it.
	requireError(t, call(t, a, true, map[string]any{"code": "a"}), "busy")
	close(store.release)
	requireError(t, <-result, "cancelled")
	requireOK(t, call(t, a, false, map[string]any{"code": "a"}))
}

type commitThenCancelStore struct{ cancel context.CancelFunc }

func (commitThenCancelStore) Get(context.Context, string) (document.Document, error) {
	panic("unexpected read")
}
func (s commitThenCancelStore) Put(_ context.Context, id, _, code string) (document.Document, error) {
	doc := document.Document{ID: id, Revision: "committed-1", Code: code}
	s.cancel()
	return doc, nil
}

func TestCommittedStoreResultSurvivesCancellationRace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := testApp(t, func(c *config) { c.Store = commitThenCancelStore{cancel: cancel} })
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"document_id":"diagram","expected_revision":"","code":"a"}`)}}
	result, err := a.StoreD2Handler(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	// Cancellation cannot undo a completed backend commit. Preserve its revision
	// so callers can reconcile the write rather than retrying an apparent failure.
	requireOK(t, result)
	if ctx.Err() != context.Canceled || decodeOutput[storeOutput](t, result).Revision != "committed-1" {
		t.Fatalf("lost committed revision: %+v", result.StructuredContent)
	}
}

type cancelAfterWrite struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *cancelAfterWrite) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data)
	w.cancel()
	return n, err
}

type shortArtifactWriter struct{}

func (shortArtifactWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestArtifactWritesObserveCancellationBetweenChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &cancelAfterWrite{cancel: cancel}
	if err := writeArtifactData(ctx, w, bytes.Repeat([]byte("x"), 128<<10)); !errors.Is(err, context.Canceled) {
		t.Fatalf("write error = %v", err)
	}
	if w.Len() != 64<<10 {
		t.Fatalf("wrote %d bytes after cancellation", w.Len())
	}
	if err := writeArtifactData(context.Background(), shortArtifactWriter{}, []byte("artifact")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write error = %v", err)
	}
}

// Cancel at an actual filesystem boundary rather than depending on timing or
// the number of context checks. The existing destination must survive replacement.
type cancelOnArtifactCreation struct {
	context.Context
	cancel       context.CancelFunc
	dir          string
	beforeCancel func()
}

func (c *cancelOnArtifactCreation) Err() error {
	entries, _ := os.ReadDir(c.dir)
	for _, entry := range entries {
		if entry.Name() == "new.svg" || entry.Name() == "diagram.svg" || strings.HasPrefix(entry.Name(), ".d2-mcp-") {
			if c.beforeCancel != nil {
				c.beforeCancel()
				c.beforeCancel = nil
			}
			c.cancel()
		}
	}
	return c.Context.Err()
}

func TestCancelledArtifactWriteCleansUpWithoutReplacing(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "replace"}[overwrite], func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "existing.svg"), []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			a := testApp(t, func(c *config) { c.OutputDir, c.AllowOverwrite = dir, true })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			boundary := &cancelOnArtifactCreation{Context: ctx, cancel: cancel, dir: dir}
			name := "new.svg"
			if overwrite {
				name = "existing.svg"
			}
			if _, err := a.writeArtifact(boundary, name, "svg", []byte("replacement"), overwrite); !errors.Is(err, context.Canceled) {
				t.Fatalf("write error = %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 || entries[0].Name() != "existing.svg" {
				t.Fatalf("artifact leaked: %v, %v", entries, err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "existing.svg"))
			if err != nil || string(data) != "original" {
				t.Fatalf("original replaced: %q, %v", data, err)
			}
		})
	}
}

func TestCancelledCreatePreservesConcurrentReplacement(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t, func(c *config) { c.OutputDir, c.AllowOverwrite = dir, true })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var replacementErr error
	committed := false
	boundary := &cancelOnArtifactCreation{Context: ctx, cancel: cancel, dir: dir, beforeCancel: func() {
		// Interleave another successful request after the canceled request has
		// created its file but before cleanup. Cleanup must only remove its temp.
		_, replacementErr = a.writeArtifact(context.Background(), "diagram.svg", "svg", []byte("successful replacement"), true)
		committed = replacementErr == nil
	}}
	if _, err := a.writeArtifact(boundary, "diagram.svg", "svg", []byte("canceled original"), false); !errors.Is(err, context.Canceled) {
		t.Fatalf("create error = %v", err)
	}
	if !committed {
		t.Fatalf("replacement was not committed: %v", replacementErr)
	}
	data, err := os.ReadFile(filepath.Join(dir, "diagram.svg"))
	if err != nil || string(data) != "successful replacement" {
		t.Fatalf("successful replacement lost: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "diagram.svg" {
		t.Fatalf("temporary artifact leaked: %v, %v", entries, err)
	}
}

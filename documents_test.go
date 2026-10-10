package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/h0rv/d2-mcp/document"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A test-only shared backend. Production must implement atomic CAS in its store,
// not copy this process-local mutex across separately running replicas.
type testStore struct {
	mu       sync.Mutex
	docs     map[string]document.Document
	revision int
}

func (s *testStore) Get(ctx context.Context, id string) (document.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return document.Document{}, err
	}
	d, ok := s.docs[id]
	if !ok {
		return d, document.ErrNotFound
	}
	return d, nil
}
func (s *testStore) Put(ctx context.Context, id, expected, code string) (document.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return document.Document{}, err
	}
	current, exists := s.docs[id]
	if exists && current.Revision != expected || !exists && expected != "" {
		return document.Document{}, document.ErrConflict
	}
	s.revision++
	d := document.Document{ID: id, Revision: fmt.Sprint(s.revision), Code: code}
	s.docs[id] = d
	return d, nil
}
func storeCall(t *testing.T, a *application, id, expected, code string) *mcp.CallToolResult {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"document_id": id, "expected_revision": expected, "code": code})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.StoreD2Handler(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: raw}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestOptionalStoreAcrossInstances(t *testing.T) {
	shared := &testStore{docs: make(map[string]document.Document)}
	a := testApp(t, func(c *config) { c.Store = shared })
	b := testApp(t, func(c *config) { c.Store = shared })
	r := storeCall(t, a, "design", "", "a -> b")
	requireOK(t, r)
	revision := r.StructuredContent.(map[string]any)["revision"].(string)
	requireOK(t, call(t, b, true, map[string]any{"document_id": "design", "revision": revision, "format": "svg"}))
	// Caller must retain an explicit revision; arbitrary result metadata is not input.
	requireError(t, call(t, b, true, map[string]any{"document_id": "design"}), "invalid_arguments")
	requireError(t, storeCall(t, b, "design", "", "a -> c"), "revision_conflict")
	r = storeCall(t, b, "design", revision, "a -> c")
	requireOK(t, r)
	requireError(t, call(t, a, true, map[string]any{"document_id": "design", "revision": revision}), "revision_conflict")
	requireError(t, storeCall(t, a, "design", revision, "a -> d"), "revision_conflict")
	next := r.StructuredContent.(map[string]any)["revision"].(string)
	// An unrelated deployment with a different backend cannot resolve the handle.
	isolated := testApp(t, func(c *config) { c.Store = &testStore{docs: make(map[string]document.Document)} })
	requireError(t, call(t, isolated, false, map[string]any{"document_id": "design", "revision": next}), "document_not_found")
	// Resubmitting complete source works regardless of backend or previous calls.
	requireOK(t, call(t, testApp(t, nil), false, map[string]any{"code": "a -> c"}))
	requireError(t, storeCall(t, a, "import", "", "...@local.d2"), "invalid_d2")
}
func TestStoreConcurrentRevision(t *testing.T) {
	shared := &testStore{docs: make(map[string]document.Document)}
	a := testApp(t, func(c *config) { c.Store = shared })
	b := testApp(t, func(c *config) { c.Store = shared })
	first := storeCall(t, a, "design", "", "a")
	requireOK(t, first)
	revision := first.StructuredContent.(map[string]any)["revision"].(string)
	results := make(chan *mcp.CallToolResult, 2)
	for _, app := range []*application{a, b} {
		go func() { results <- storeCall(t, app, "design", revision, "a -> b") }()
	}
	success := 0
	for range 2 {
		r := <-results
		if !r.IsError {
			success++
		} else {
			requireError(t, r, "revision_conflict")
		}
	}
	if success != 1 {
		t.Fatalf("successful writes=%d; want 1", success)
	}
}

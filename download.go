package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// downloadItem represents a temporary file available for download
type downloadItem struct {
	Data        []byte
	ContentType string
	Filename    string
	ExpiresAt   time.Time
}

// downloadStore manages temporary downloads
type downloadStore struct {
	mu    sync.RWMutex
	items map[string]*downloadItem
}

var downloads = &downloadStore{
	items: make(map[string]*downloadItem),
}

// generateID creates a random download ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Store adds a download and returns its ID
func (ds *downloadStore) Store(data []byte, contentType, filename string, ttl time.Duration) string {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	id := generateID()
	ds.items[id] = &downloadItem{
		Data:        data,
		ContentType: contentType,
		Filename:    filename,
		ExpiresAt:   time.Now().Add(ttl),
	}

	return id
}

// Get retrieves a download by ID
func (ds *downloadStore) Get(id string) (*downloadItem, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	item, exists := ds.items[id]
	if !exists || time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item, true
}

// Delete removes a download by ID
func (ds *downloadStore) Delete(id string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.items, id)
}

// Cleanup removes expired downloads
func (ds *downloadStore) Cleanup() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	now := time.Now()
	for id, item := range ds.items {
		if now.After(item.ExpiresAt) {
			delete(ds.items, id)
		}
	}
}

// StartCleanupWorker starts a background goroutine to cleanup expired downloads
func (ds *downloadStore) StartCleanupWorker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ds.Cleanup()
		}
	}()
}

// ServeDownload handles HTTP download requests
func ServeDownload(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /download/{id}
	path := r.URL.Path
	if len(path) < 11 { // "/download/" is 10 chars
		http.Error(w, "Invalid download URL", http.StatusBadRequest)
		return
	}

	id := path[10:] // Everything after "/download/"

	// Get the download
	item, exists := downloads.Get(id)
	if !exists {
		http.Error(w, "Download not found or expired", http.StatusNotFound)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", item.Filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(item.Data)))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Write the file data
	w.Write(item.Data)

	// Delete after serving (one-time download)
	go downloads.Delete(id)
}

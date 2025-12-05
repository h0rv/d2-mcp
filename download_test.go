package main

import (
	"testing"
	"time"
)

func TestDownloadStore(t *testing.T) {
	store := &downloadStore{
		items: make(map[string]*downloadItem),
	}

	// Test Store
	data := []byte("test data")
	id := store.Store(data, "text/plain", "test.txt", 1*time.Second)

	if id == "" {
		t.Error("Store should return non-empty ID")
	}

	// Test Get
	item, exists := store.Get(id)
	if !exists {
		t.Error("Item should exist after storing")
	}

	if string(item.Data) != "test data" {
		t.Errorf("Expected 'test data', got %s", string(item.Data))
	}

	if item.ContentType != "text/plain" {
		t.Errorf("Expected 'text/plain', got %s", item.ContentType)
	}

	if item.Filename != "test.txt" {
		t.Errorf("Expected 'test.txt', got %s", item.Filename)
	}

	// Test Delete
	store.Delete(id)
	_, exists = store.Get(id)
	if exists {
		t.Error("Item should not exist after deletion")
	}

	// Test expiration
	id2 := store.Store(data, "text/plain", "test2.txt", 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	_, exists = store.Get(id2)
	if exists {
		t.Error("Item should not exist after expiration")
	}

	// Test Cleanup
	id3 := store.Store(data, "text/plain", "test3.txt", 50*time.Millisecond)
	id4 := store.Store(data, "text/plain", "test4.txt", 10*time.Second)

	time.Sleep(100 * time.Millisecond)
	store.Cleanup()

	_, exists = store.Get(id3)
	if exists {
		t.Error("Expired item should be cleaned up")
	}

	_, exists = store.Get(id4)
	if !exists {
		t.Error("Non-expired item should still exist after cleanup")
	}
}

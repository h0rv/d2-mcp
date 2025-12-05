package main

import (
	"testing"
)

func TestAppsToolMeta_Structure(t *testing.T) {
	meta := appsResourceMeta()

	if meta == nil {
		t.Fatal("appsResourceMeta returned nil")
	}

	// Verify all expected fields are present
	expectedFields := []string{
		"openai/outputTemplate",
		"openai/widgetAccessible",
		"openai/resultCanProduceWidget",
		"openai/toolInvocation/invoking",
		"openai/toolInvocation/invoked",
	}

	for _, field := range expectedFields {
		if _, ok := meta[field]; !ok {
			t.Errorf("expected field %q not found in metadata", field)
		}
	}

	// Check specific values
	if meta["openai/outputTemplate"] != appsResourceURI {
		t.Errorf("expected openai/outputTemplate to be %q, got %v", appsResourceURI, meta["openai/outputTemplate"])
	}

	if meta["openai/widgetAccessible"] != true {
		t.Errorf("expected openai/widgetAccessible to be true, got %v", meta["openai/widgetAccessible"])
	}

	if meta["openai/resultCanProduceWidget"] != true {
		t.Errorf("expected openai/resultCanProduceWidget to be true, got %v", meta["openai/resultCanProduceWidget"])
	}
}

func TestLoadAppsViewerTemplate(t *testing.T) {
	html, err := loadAppsViewerTemplate()
	if err != nil {
		t.Fatalf("loadAppsViewerTemplate failed: %v", err)
	}

	if html == "" {
		t.Error("expected non-empty HTML template")
	}

	// Check that it contains expected elements (case insensitive)
	if !contains(html, "<!DOCTYPE html>") && !contains(html, "<!doctype html>") {
		t.Error("HTML should contain DOCTYPE")
	}

	if !contains(html, "window.openai") {
		t.Error("HTML should reference window.openai for Apps SDK integration")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBoolPtr(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)

	if truePtr == nil || *truePtr != true {
		t.Error("boolPtr(true) should return pointer to true")
	}

	if falsePtr == nil || *falsePtr != false {
		t.Error("boolPtr(false) should return pointer to false")
	}
}

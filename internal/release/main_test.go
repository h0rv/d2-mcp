package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseInputs(t *testing.T) {
	for _, version := range []string{"1.3.0", "v1.3.0", "1.3.0-rc.1"} {
		if !releaseVersion.MatchString(version) {
			t.Errorf("rejected %q", version)
		}
	}
	for _, version := range []string{"", "main", "1.2", "1.2.3\n", "1.2.3 -X main.other=x", "$(command)", "1.2.3/extra"} {
		if releaseVersion.MatchString(version) {
			t.Errorf("accepted %q", version)
		}
	}
	dir := t.TempDir()
	for _, name := range assets {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := checksums(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range assets {
		if !strings.Contains(string(data), fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(name)), name)) {
			t.Fatalf("missing or incorrect digest for %s", name)
		}
	}
	if err := os.Remove(filepath.Join(dir, assets[0])); err != nil {
		t.Fatal(err)
	}
	if err := checksums(dir); err == nil {
		t.Fatal("published an incomplete release manifest")
	}
}

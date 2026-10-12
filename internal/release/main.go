// Command release validates a release tag and optionally writes SHA256SUMS.
// It does not download, install or publish anything.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var releaseVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)

var assets = []string{
	"d2-mcp-darwin-amd64", "d2-mcp-darwin-arm64",
	"d2-mcp-linux-amd64", "d2-mcp-linux-arm64", "d2-mcp-windows-amd64.exe",
}

func checksums(directory string) error {
	var lines strings.Builder
	for _, name := range assets {
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("%s must be a nonempty regular file", name)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Fprintf(&lines, "%x  %s\n", hash.Sum(nil), name)
	}
	return os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(lines.String()), 0644)
}

func main() {
	version := flag.String("version", "", "Exact release tag to validate")
	directory := flag.String("dir", "", "Directory containing all five binaries; writes SHA256SUMS when set")
	flag.Parse()
	if !releaseVersion.MatchString(*version) || flag.NArg() != 0 {
		log.Fatal("use an exact release version, such as 1.3.0 or 1.3.0-rc.1")
	}
	if *directory != "" {
		if err := checksums(*directory); err != nil {
			log.Fatal(err)
		}
	}
}

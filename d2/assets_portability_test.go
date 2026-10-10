package d2

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"runtime"
	"testing"
	"testing/fstest"
)

func TestPNGWithoutWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not permit removal of the current directory")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	prepared, err := Prepare(ctx, Source{Code: "a -> b", Path: "input.d2", FS: fstest.MapFS{}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := Render(ctx, prepared, "png", "extended")
	if err != nil {
		t.Fatal(err)
	}
	frame, err := png.Decode(bytes.NewReader(data))
	if err != nil || frame.Bounds().Dx() < 20 || frame.Bounds().Dy() < 20 {
		t.Fatalf("invalid PNG without working directory: %v", err)
	}
}

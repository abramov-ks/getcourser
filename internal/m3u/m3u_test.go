package m3u

import (
	"os"
	"testing"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "m3u_test_*.m3u")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestGetSlices_Basic(t *testing.T) {
	content := `#EXTM3U
#EXTINF:10,
https://example.com/chunk1.ts
#EXTINF:10,
https://example.com/chunk2.ts
`
	path := writeTempFile(t, content)
	slices, err := New(path).GetSlices()
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0] != "https://example.com/chunk1.ts" {
		t.Errorf("unexpected slice[0]: %q", slices[0])
	}
	if slices[1] != "https://example.com/chunk2.ts" {
		t.Errorf("unexpected slice[1]: %q", slices[1])
	}
}

func TestGetSlices_Empty(t *testing.T) {
	content := `#EXTM3U
# no segments here
`
	path := writeTempFile(t, content)
	slices, err := New(path).GetSlices()
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 0 {
		t.Fatalf("expected 0 slices, got %d", len(slices))
	}
}

func TestGetSlices_SkipsBlankURLAfterExtinf(t *testing.T) {
	content := `#EXTM3U
#EXTINF:10,

#EXTINF:10,
https://example.com/chunk.ts
`
	path := writeTempFile(t, content)
	slices, err := New(path).GetSlices()
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
}

func TestGetSlices_FileNotFound(t *testing.T) {
	_, err := New("/nonexistent/path/file.m3u").GetSlices()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestGetSlices_RelativeURLs(t *testing.T) {
	content := `#EXTM3U
#EXTINF:10,
chunk1.ts
#EXTINF:10,
chunk2.ts
`
	path := writeTempFile(t, content)
	slices, err := New(path).GetSlices()
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0] != "chunk1.ts" {
		t.Errorf("unexpected slice[0]: %q", slices[0])
	}
}

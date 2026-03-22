package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// getPlaylistFileVideos
// ---------------------------------------------------------------------------

func writePlaylistFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "playlist_test_*.playlist")
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

func TestGetPlaylistFileVideos_NameAndURL(t *testing.T) {
	content := "video1.mpg https://example.com/v1.m3u8\nvideo2.mpg https://example.com/v2.m3u8\n"
	path := writePlaylistFile(t, content)

	lines, err := getPlaylistFileVideos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(lines))
	}
	if lines["video1.mpg"] != "https://example.com/v1.m3u8" {
		t.Errorf("unexpected value for video1.mpg: %q", lines["video1.mpg"])
	}
	if lines["video2.mpg"] != "https://example.com/v2.m3u8" {
		t.Errorf("unexpected value for video2.mpg: %q", lines["video2.mpg"])
	}
}

func TestGetPlaylistFileVideos_URLOnly(t *testing.T) {
	content := "https://example.com/v1.m3u8\nhttps://example.com/v2.m3u8\n"
	path := writePlaylistFile(t, content)

	lines, err := getPlaylistFileVideos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(lines))
	}
	// Keys should be auto-generated as "video_1", "video_2"
	if _, ok := lines["video_1"]; !ok {
		t.Error("expected key video_1")
	}
	if _, ok := lines["video_2"]; !ok {
		t.Error("expected key video_2")
	}
}

func TestGetPlaylistFileVideos_Mixed(t *testing.T) {
	content := "named.mpg https://example.com/v1.m3u8\nhttps://example.com/v2.m3u8\n"
	path := writePlaylistFile(t, content)

	lines, err := getPlaylistFileVideos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(lines))
	}
	if lines["named.mpg"] != "https://example.com/v1.m3u8" {
		t.Errorf("unexpected value for named.mpg: %q", lines["named.mpg"])
	}
	if _, ok := lines["video_1"]; !ok {
		t.Error("expected key video_1")
	}
}

func TestGetPlaylistFileVideos_FileNotFound(t *testing.T) {
	_, err := getPlaylistFileVideos("/nonexistent/path.playlist")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestGetPlaylistFileVideos_Empty(t *testing.T) {
	path := writePlaylistFile(t, "")
	lines, err := getPlaylistFileVideos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// resolveURL
// ---------------------------------------------------------------------------

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestResolveURL_AbsoluteRef(t *testing.T) {
	base := mustParseURL(t, "https://example.com/path/playlist.m3u8")
	result := resolveURL(base, "https://cdn.example.com/chunk.ts")
	if result != "https://cdn.example.com/chunk.ts" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestResolveURL_RelativeRef(t *testing.T) {
	base := mustParseURL(t, "https://example.com/path/playlist.m3u8")
	result := resolveURL(base, "chunk.ts")
	if result != "https://example.com/path/chunk.ts" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestResolveURL_RelativeRefWithSubdir(t *testing.T) {
	base := mustParseURL(t, "https://example.com/videos/hls/playlist.m3u8")
	result := resolveURL(base, "seg/chunk.ts")
	if result != "https://example.com/videos/hls/seg/chunk.ts" {
		t.Errorf("unexpected result: %q", result)
	}
}

// ---------------------------------------------------------------------------
// createUnusedFilename
// ---------------------------------------------------------------------------

func TestCreateUnusedFilename_NoConflict(t *testing.T) {
	a := &App{}
	name := filepath.Join(t.TempDir(), "output.mpg")
	result := a.createUnusedFilename(name)
	if result != name {
		t.Errorf("expected %q, got %q", name, result)
	}
}

func TestCreateUnusedFilename_WithConflict(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "output.mpg")

	// Create the base file so there is a conflict.
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	a := &App{}
	result := a.createUnusedFilename(name)
	expected := filepath.Join(dir, "output(1).mpg")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCreateUnusedFilename_MultipleConflicts(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "output.mpg")

	for _, n := range []string{base, filepath.Join(dir, "output(1).mpg")} {
		f, err := os.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	a := &App{}
	result := a.createUnusedFilename(base)
	expected := filepath.Join(dir, "output(2).mpg")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ---------------------------------------------------------------------------
// getSliceByNo
// ---------------------------------------------------------------------------

func TestGetSliceByNo_Found(t *testing.T) {
	a := &App{}
	slices := []DownloadedVideoChunk{
		{i: 0, path: "a.ts"},
		{i: 1, path: "b.ts"},
		{i: 2, path: "c.ts"},
	}
	result := a.getSliceByNo(slices, 1)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.path != "b.ts" {
		t.Errorf("unexpected path: %q", result.path)
	}
}

func TestGetSliceByNo_NotFound(t *testing.T) {
	a := &App{}
	slices := []DownloadedVideoChunk{{i: 0, path: "a.ts"}}
	result := a.getSliceByNo(slices, 5)
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// reorderSlicesToArray
// ---------------------------------------------------------------------------

func TestReorderSlicesToArray_OutOfOrder(t *testing.T) {
	a := &App{}
	slices := []DownloadedVideoChunk{
		{i: 2, path: "c.ts"},
		{i: 0, path: "a.ts"},
		{i: 1, path: "b.ts"},
	}
	files, err := a.reorderSlicesToArray(slices)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"a.ts", "b.ts", "c.ts"}
	for i, f := range files {
		if f != expected[i] {
			t.Errorf("index %d: expected %q, got %q", i, expected[i], f)
		}
	}
}

func TestReorderSlicesToArray_MissingIndex(t *testing.T) {
	a := &App{}
	slices := []DownloadedVideoChunk{
		{i: 0, path: "a.ts"},
		{i: 2, path: "c.ts"}, // index 1 is missing
	}
	_, err := a.reorderSlicesToArray(slices)
	if err == nil {
		t.Fatal("expected error for missing index, got nil")
	}
}

// ---------------------------------------------------------------------------
// concatFiles
// ---------------------------------------------------------------------------

func TestConcatFiles(t *testing.T) {
	dir := t.TempDir()

	// Create two source chunk files.
	chunk1 := filepath.Join(dir, "c1.ts")
	chunk2 := filepath.Join(dir, "c2.ts")
	if err := os.WriteFile(chunk1, []byte("AAAA"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chunk2, []byte("BBBB"), 0644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(dir, "out.mpg")
	outFile, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if err := a.concatFiles(outFile, []string{chunk1, chunk2}); err != nil {
		t.Fatal(err)
	}
	outFile.Close()

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "AAAABBBB" {
		t.Errorf("unexpected output: %q", string(data))
	}
}

func TestConcatFiles_MissingChunk(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.mpg")
	outFile, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer outFile.Close()

	a := &App{}
	err = a.concatFiles(outFile, []string{"/nonexistent/chunk.ts"})
	if err == nil {
		t.Fatal("expected error for missing chunk, got nil")
	}
}

// ---------------------------------------------------------------------------
// NewApp
// ---------------------------------------------------------------------------

func TestNewApp(t *testing.T) {
	pl := "batch.playlist"
	a := NewApp("https://example.com/v.m3u8", 8, true, &pl, "out.mpg")
	if a.Url != "https://example.com/v.m3u8" {
		t.Errorf("unexpected Url: %q", a.Url)
	}
	if a.Threads != 8 {
		t.Errorf("unexpected Threads: %d", a.Threads)
	}
	if !a.Verbose {
		t.Error("expected Verbose to be true")
	}
	if *a.playlistFile != pl {
		t.Errorf("unexpected playlistFile: %q", *a.playlistFile)
	}
	if a.outputName != "out.mpg" {
		t.Errorf("unexpected outputName: %q", a.outputName)
	}
}

// Ensure fmt is used (some test helpers use it indirectly via errors).
var _ = fmt.Sprintf

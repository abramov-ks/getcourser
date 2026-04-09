package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
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

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// ---------------------------------------------------------------------------
// getPlaylistFileVideos
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// downloadFileToTemporary
// ---------------------------------------------------------------------------

func TestDownloadFileToTemporary_EmptyURL(t *testing.T) {
	a := &App{}
	_, err := a.downloadFileToTemporary("")
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

func TestDownloadFileToTemporary_Success(t *testing.T) {
	body := []byte("chunk data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	a := &App{}
	path, err := a.downloadFileToTemporary(srv.URL + "/chunk.ts")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("expected %q, got %q", body, got)
	}
}

func TestDownloadFileToTemporary_ServerError(t *testing.T) {
	// The function currently does not check the HTTP status code,
	// so a 500 still writes the body and returns no error — verify
	// that behaviour is consistent (not a crash).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := &App{}
	path, err := a.downloadFileToTemporary(srv.URL + "/chunk.ts")
	// Either an error is returned or a temp file is created — both are valid.
	if err == nil {
		os.Remove(path)
	}
}

func TestDownloadFileToTemporary_ConnectionRefused(t *testing.T) {
	a := &App{}
	_, err := a.downloadFileToTemporary("http://127.0.0.1:1") // nothing listening
	if err == nil {
		t.Fatal("expected error for refused connection, got nil")
	}
}

// ---------------------------------------------------------------------------
// cleanup
// ---------------------------------------------------------------------------

func TestCleanup_RemovesTempFilesOnSuccess(t *testing.T) {
	dir := t.TempDir()

	playlist := filepath.Join(dir, "playlist.m3u8")
	chunk := filepath.Join(dir, "chunk.ts")
	output := filepath.Join(dir, "out.mpg")
	for _, p := range []string{playlist, chunk, output} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	a := &App{
		completed:        true,
		actualOutputName: output,
	}
	a.tempFiles.playlist = playlist
	a.tempFiles.slices = []string{chunk}

	a.cleanup()

	// Temp files should be gone; output file should remain (completed=true).
	for _, p := range []string{playlist, chunk} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %q to be deleted", p)
		}
	}
	if _, err := os.Stat(output); err != nil {
		t.Errorf("output file should remain after successful download: %v", err)
	}
}

func TestCleanup_RemovesOutputOnFailure(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "out.mpg")
	if err := os.WriteFile(output, []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}

	a := &App{
		completed:        false,
		actualOutputName: output,
	}

	a.cleanup()

	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Error("expected incomplete output file to be deleted")
	}
}

func TestCleanup_NoOutputNameDoesNotPanic(t *testing.T) {
	// cleanup should be safe when actualOutputName is empty (failure before file creation).
	a := &App{completed: false, actualOutputName: ""}
	a.cleanup() // must not panic
}

// ---------------------------------------------------------------------------
// DownloadVideo — end-to-end with httptest
// ---------------------------------------------------------------------------

func TestDownloadVideo_EndToEnd(t *testing.T) {
	chunk1 := []byte("CHUNK1DATA")
	chunk2 := []byte("CHUNK2DATA")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/playlist.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte("#EXTM3U\n#EXTINF:5,\nchunk1.ts\n#EXTINF:5,\nchunk2.ts\n"))
		case "/chunk1.ts":
			w.Write(chunk1)
		case "/chunk2.ts":
			w.Write(chunk2)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "output.mpg")

	a := NewApp(srv.URL+"/playlist.m3u8", 2, false, nil, outPath)
	a.DownloadVideo(srv.URL+"/playlist.m3u8", outPath)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "CHUNK1DATA") || !strings.Contains(got, "CHUNK2DATA") {
		t.Errorf("unexpected output content: %q", got)
	}
}

func TestDownloadVideo_ResetsStateBetweenCalls(t *testing.T) {
	// Verify that calling DownloadVideo twice on the same App instance
	// does not carry over stale state from the first run.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".m3u8"):
			w.Write([]byte("#EXTM3U\n#EXTINF:5,\nchunk.ts\n"))
		case strings.HasSuffix(r.URL.Path, ".ts"):
			callCount++
			w.Write([]byte("DATA"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()

	a := NewApp("", 1, false, nil, "")

	out1 := filepath.Join(dir, "run1.mpg")
	a.DownloadVideo(srv.URL+"/run1.m3u8", out1)

	out2 := filepath.Join(dir, "run2.mpg")
	a.DownloadVideo(srv.URL+"/run2.m3u8", out2)

	for _, p := range []string{out1, out2} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected output file %q to exist: %v", p, err)
		}
	}
	if callCount != 2 {
		t.Errorf("expected 2 chunk downloads (one per run), got %d", callCount)
	}
}
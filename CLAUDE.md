# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build

```bash
make build        # builds the grabber binary
go build ./cmd/grabber
```

## Usage

```bash
# Single video download
grabber <playlist_url> <output.mpg>

# Batch download from playlist file
grabber -playlist=<file.playlist> <url>

# Options
grabber -threads=10 -verbose <playlist_url> <output.mpg>
```

**Playlist file format** (`.playlist`):
```
filename.mpg https://example.com/playlist.m3u8
https://example.com/other.m3u8
```
Lines with a URL only auto-generate filenames as `video_N`.

## Architecture

The app downloads M3U/HLS video playlists by fetching all chunks in parallel and concatenating them.

**Flow:**
1. `cmd/grabber/main.go` — CLI flags parsing, dispatches to `App`
2. `internal/app/app.go` — Core logic:
   - Downloads M3U playlist to a temp file
   - Parses chunk URLs via `internal/m3u`
   - Runs a worker pool (goroutines reading from `jobsChan`) to download chunks in parallel
   - Reorders downloaded chunks by index before concatenation
   - Concatenates all chunks into the output file, then cleans up temp files
3. `internal/m3u/m3u.go` — Parses M3U format; extracts URLs following `#EXTINF` markers

**Key types:**
- `App` — holds config (threads, verbose, playlist path) and channel-based worker pool state
- `Job` / `Result` — passed through `jobsChan` / `resultsChan` to coordinate parallel downloads
- Temp files are named with an MD5 hash of the source URL to avoid collisions
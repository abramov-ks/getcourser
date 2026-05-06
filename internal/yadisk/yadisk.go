package yadisk

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Stream represents a single video stream with its quality dimension and M3U8 URL.
type Stream struct {
	Dimension string
	URL       string
}

// streamRe matches entries like: {"dimension":"720p","size":{...},"url":"https://..."}
var streamRe = regexp.MustCompile(`\{"dimension":"([^"]+)","size":\{[^}]*\},"url":"([^"]+)"`)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// FetchStreams downloads the Yandex Disk share page and returns all available streams.
func FetchStreams(pageURL string) ([]Stream, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading page body: %w", err)
	}

	return parseStreams(string(body))
}

func parseStreams(html string) ([]Stream, error) {
	matches := streamRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no video streams found in page; make sure the URL is a Yandex Disk video share link")
	}

	streams := make([]Stream, 0, len(matches))
	for _, m := range matches {
		streams = append(streams, Stream{Dimension: m[1], URL: m[2]})
	}
	return streams, nil
}

// SelectBest picks the stream matching the requested quality string (e.g. "720p").
// If quality is empty it selects the stream with the highest numeric resolution,
// skipping "adaptive" and other non-numeric dimensions.
func SelectBest(streams []Stream, quality string) (*Stream, error) {
	if quality != "" {
		for i := range streams {
			if streams[i].Dimension == quality {
				return &streams[i], nil
			}
		}
		return nil, fmt.Errorf("quality %q not found; available: %s", quality, joinDimensions(streams))
	}

	// Auto-select: highest numeric resolution.
	var best *Stream
	bestN := -1
	for i := range streams {
		n := parseResolution(streams[i].Dimension)
		if n > bestN {
			bestN = n
			best = &streams[i]
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no suitable stream found among: %s", joinDimensions(streams))
	}
	return best, nil
}

// parseResolution converts "720p" -> 720; non-numeric dimensions return -1.
func parseResolution(dim string) int {
	s := strings.TrimSuffix(dim, "p")
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

func joinDimensions(streams []Stream) string {
	dims := make([]string, 0, len(streams))
	for _, s := range streams {
		dims = append(dims, s.Dimension)
	}
	sort.Strings(dims)
	return strings.Join(dims, ", ")
}
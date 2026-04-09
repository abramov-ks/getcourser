package app

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"getcourser/internal/m3u"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
)

type App struct {
	Url          string
	Threads      int
	Verbose      bool
	playlistFile *string
	outputName   string
	tempFiles    struct {
		playlist string
		slices   []string
	}
	completed        bool
	actualOutputName string
}

type VideoChunk struct {
	i     int
	url   string
	total int
}

type DownloadedVideoChunk struct {
	i    int
	path string
}

func NewApp(url string, threads int, verboseMode bool, playlistFile *string, outputName string) *App {
	return &App{
		Url:          url,
		Threads:      threads,
		Verbose:      verboseMode,
		playlistFile: playlistFile,
		outputName:   outputName,
	}
}

func (a *App) Run() {
	if a.playlistFile == nil || *a.playlistFile == "" {
		a.DownloadVideo(a.Url, a.outputName)
		return
	}

	lines, err := getPlaylistFileVideos(*a.playlistFile)
	if err != nil {
		log.Fatal(err)
		return
	}

	for name, line := range lines {
		a.DownloadVideo(line, name)
	}
}

func getPlaylistFileVideos(playlistFile string) (map[string]string, error) {
	lines := make(map[string]string)
	cntr := 1
	file, err := os.Open(playlistFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, " ")
		if len(splits) != 2 {
			lines[fmt.Sprintf("video_%d", cntr)] = splits[0]
			cntr++
		} else {
			lines[splits[0]] = splits[1]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func (a *App) DownloadVideo(playlistURL string, fname string) {
	// Reset per-run state so playlist-mode calls start clean.
	a.completed = false
	a.actualOutputName = ""
	a.tempFiles.playlist = ""
	a.tempFiles.slices = nil

	defer a.cleanup()

	fmt.Printf("[+] Downloading playlist...\n")

	playlistFile, err := a.downloadFileToTemporary(playlistURL)
	if err != nil {
		fmt.Println("[!] Error downloading playlist file:", err)
		return
	}
	a.tempFiles.playlist = playlistFile

	if a.Verbose {
		fmt.Printf("[+] Start reading playlist...\n")
	}

	m3uReader := m3u.New(playlistFile)
	slicesToDownload, err := m3uReader.GetSlices()
	if err != nil {
		fmt.Println("[!] Error getting m3u:", err)
		return
	}

	if len(slicesToDownload) < 1 {
		fmt.Println("[!] No m3u chunks found")
		return
	}

	fmt.Printf("[+] Found %d chunks to download\n", len(slicesToDownload))

	baseURL, err := url.Parse(playlistURL)
	if err != nil {
		fmt.Println("[!] Error parsing playlist URL:", err)
		return
	}

	numJobs := len(slicesToDownload)
	jobsChan := make(chan VideoChunk, numJobs)
	resultsChan := make(chan DownloadedVideoChunk, numJobs)

	fmt.Printf("[+] Start downloading with %d threads\n", a.Threads)
	for w := 1; w <= a.Threads; w++ {
		go a.worker(jobsChan, resultsChan)
	}

	bar := a.createProgressBar(numJobs)

	for i, slice := range slicesToDownload {
		jobsChan <- VideoChunk{i: i, url: resolveURL(baseURL, slice), total: numJobs}
	}
	close(jobsChan)

	downloadedSlices := make([]DownloadedVideoChunk, 0, numJobs)
	for i := 0; i < numJobs; i++ {
		downloadedSlices = append(downloadedSlices, <-resultsChan)
		bar.Add(1)
	}

	for _, downloadedSlice := range downloadedSlices {
		a.tempFiles.slices = append(a.tempFiles.slices, downloadedSlice.path)
	}

	outputPath := a.createUnusedFilename(fname)
	a.actualOutputName = outputPath

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("[!] Error opening output file:", err)
		return
	}
	defer outputFile.Close()

	files, err := a.reorderSlicesToArray(downloadedSlices)
	if err != nil {
		fmt.Println("[!] Error ordering slices:", err)
		return
	}

	if err = a.concatFiles(outputFile, files); err != nil {
		fmt.Println("[!] Error concatenating chunks:", err)
		return
	}

	fmt.Printf("[+] Done. Downloaded %d chunks to %s \n", len(downloadedSlices), outputFile.Name())
	a.completed = true
}

func (a *App) worker(jobs <-chan VideoChunk, results chan<- DownloadedVideoChunk) {
	for job := range jobs {
		path, err := a.downloadFileToTemporary(job.url)
		if err != nil && a.Verbose {
			fmt.Printf("[!] Error downloading chunk %d: %v\n", job.i, err)
		}
		results <- DownloadedVideoChunk{i: job.i, path: path}
	}
}

func resolveURL(base *url.URL, ref string) string {
	refURL, err := url.Parse(ref)
	if err != nil || refURL.IsAbs() {
		return ref
	}
	return base.ResolveReference(refURL).String()
}

func (a *App) downloadFileToTemporary(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("url is empty")
	}
	hasher := md5.New()
	hasher.Write([]byte(rawURL))
	tempFile, err := os.CreateTemp("", hex.EncodeToString(hasher.Sum(nil)))
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if _, err = io.Copy(tempFile, resp.Body); err != nil {
		return "", err
	}
	return tempFile.Name(), nil
}

func (a *App) cleanup() {
	if a.Verbose {
		fmt.Printf("[+] Cleaning up...\n")
	}

	if !a.completed && a.actualOutputName != "" {
		if err := os.Remove(a.actualOutputName); err == nil && a.Verbose {
			fmt.Printf("[+] Removed drafted output file\n")
		}
	}

	if a.tempFiles.playlist != "" {
		if err := os.Remove(a.tempFiles.playlist); err == nil && a.Verbose {
			fmt.Printf("[+] Deleted playlist: %s\n", a.tempFiles.playlist)
		}
	}
	for _, slice := range a.tempFiles.slices {
		if err := os.Remove(slice); err == nil && a.Verbose {
			fmt.Printf("[+] Deleted chunk: %s\n", slice)
		}
	}
}

func (a *App) concatFiles(outputFile *os.File, slices []string) error {
	for _, slice := range slices {
		sliceFile, err := os.Open(slice)
		if err != nil {
			return fmt.Errorf("could not open slice file %v", slice)
		}
		_, err = io.Copy(outputFile, sliceFile)
		sliceFile.Close()
		if err != nil {
			return fmt.Errorf("could not copy slice file %v", slice)
		}
	}
	return nil
}

func (a *App) getSliceByNo(slices []DownloadedVideoChunk, i int) *DownloadedVideoChunk {
	for _, slice := range slices {
		if slice.i == i {
			return &slice
		}
	}
	return nil
}

func (a *App) createUnusedFilename(filename string) string {
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		return filename
	}
	extension := filepath.Ext(filename)
	basename := filename[0 : len(filename)-len(extension)]
	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s(%d)%s", basename, i, extension)
		if _, err := os.Stat(newName); errors.Is(err, os.ErrNotExist) {
			return newName
		}
	}
}

func (a *App) reorderSlicesToArray(slices []DownloadedVideoChunk) ([]string, error) {
	files := make([]string, 0, len(slices))
	for i := 0; i < len(slices); i++ {
		currentSlice := a.getSliceByNo(slices, i)
		if currentSlice == nil {
			return nil, fmt.Errorf("could not find slice #%d", i)
		}
		files = append(files, currentSlice.path)
	}
	return files, nil
}

func (a *App) createProgressBar(counter int) *progressbar.ProgressBar {
	return progressbar.NewOptions(
		counter,
		progressbar.OptionSetDescription("[+] Download chunks"),
		progressbar.OptionShowBytes(false),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetElapsedTime(false),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionShowCount(),
	)
}

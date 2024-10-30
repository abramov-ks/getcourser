package app

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"getcourser/internal/m3u"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
)

type App struct {
	Url        string
	Threads    int
	Verbose    bool
	outputName string
	tempFiles  struct {
		playlist string
		slices   []string
	}
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

func NewApp(url string, threads int, verboseMode bool, outputName string) *App {
	return &App{
		Url:        url,
		Threads:    threads,
		Verbose:    verboseMode,
		outputName: outputName,
	}
}

func (a *App) Run() {
	defer a.cleanup()

	fmt.Printf("[+] Downloading playlist...\n")

	playlistFile, err := a.downloadFileToTemporary(a.Url)
	if err != nil {
		fmt.Println("[!] Error downloading playlist file:", err)
		return
	}

	if a.Verbose {
		fmt.Printf("[+] Start reading playlist...\n")
	}

	a.tempFiles.playlist = playlistFile

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

	numWorkers := len(slicesToDownload)
	jobsChan := make(chan VideoChunk, numWorkers)
	resultsChan := make(chan DownloadedVideoChunk, numWorkers)

	downloadedSlices := make([]DownloadedVideoChunk, 0)

	fmt.Printf("[+] Start downloading with %d threads\n", a.Threads)
	for w := 1; w <= a.Threads; w++ {
		go a.worker(jobsChan, resultsChan)
	}

	bar := a.createProgressBar(len(slicesToDownload))

	for i, slice := range slicesToDownload {
		jobsChan <- VideoChunk{i: i, url: slice, total: len(slicesToDownload)}
	}
	close(jobsChan)

	for a := 1; a <= numWorkers; a++ {
		downloadedSlices = append(downloadedSlices, <-resultsChan)
		bar.Add(1)
	}

	for _, downloadedSlice := range downloadedSlices {
		a.tempFiles.slices = append(a.tempFiles.slices, downloadedSlice.path)
	}

	outputFile, err := os.OpenFile(a.createUnusedFilename(a.outputName), os.O_CREATE|os.O_WRONLY, 0644)
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

	err = a.concatFiles(outputFile, files)
	if err != nil {
		fmt.Println("[!] Error concatenating chunks:", err)
		return
	}

	fmt.Printf("[+] Done. Downloaded %d chunks to %s \n", len(downloadedSlices), outputFile.Name())
}

func (a *App) worker(slicesToDownload <-chan VideoChunk, results chan<- DownloadedVideoChunk) {
	for sliceToDownload := range slicesToDownload {
		downloadedFilePath, _ := a.downloadFileToTemporary(sliceToDownload.url)
		results <- DownloadedVideoChunk{i: sliceToDownload.i, path: downloadedFilePath}
	}
}

func (a *App) downloadFileToTemporary(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("url is empty")
	}
	hasher := md5.New()
	hasher.Write([]byte(url))
	tempFile, err := os.CreateTemp("", hex.EncodeToString(hasher.Sum(nil)))
	defer tempFile.Close()

	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(tempFile, resp.Body)

	if err != nil {
		return "", err
	}
	return tempFile.Name(), nil
}

func (a *App) cleanup() {
	if a.Verbose {
		fmt.Printf("[+] Cleaning up...\n")
	}
	err := os.Remove(a.tempFiles.playlist)
	if err == nil && a.Verbose {
		fmt.Printf("[+] Deleted playlist: %s\n", a.tempFiles.playlist)
	}
	for _, slice := range a.tempFiles.slices {
		err := os.Remove(slice)
		if err == nil && a.Verbose {
			fmt.Printf("[+] Deleted chunk: %s\n", slice)
		}
	}
}

func (a *App) concatFiles(outputFile *os.File, slices []string) error {
	for _, slice := range slices {
		sliceFile, err := os.Open(slice)
		defer sliceFile.Close()

		if err != nil {
			return fmt.Errorf("could not open slice file %v", slice)
		}
		_, err = io.Copy(outputFile, sliceFile)
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
	} else {
		var i = 1
		var extension = filepath.Ext(filename)
		var basename = filename[0 : len(filename)-len(extension)]

		for {
			var newName = fmt.Sprintf("%s(%d)%s", basename, i, extension)
			if _, err := os.Stat(newName); errors.Is(err, os.ErrNotExist) {
				return newName
			}
			i++
		}
	}
}

func (a *App) reorderSlicesToArray(slices []DownloadedVideoChunk) ([]string, error) {
	var files = make([]string, 0)
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

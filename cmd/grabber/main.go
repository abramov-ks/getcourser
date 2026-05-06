package main

import (
	"flag"
	"fmt"
	"getcourser/internal/app"
	"getcourser/internal/yadisk"
	"log"
	"os"
)

const appName = "get-course-grabber"
const defaultOutputFileName = "video.mpg"

var (
	threads     = flag.Int("threads", 5, "Number of threads to use")
	verboseMode = flag.Bool("verbose", false, "Verbose mode")
	playlist    = flag.String("playlist", "", "Batch from playlist")
	yadiskURL   = flag.String("yadisk", "", "Yandex Disk share page URL to extract playlist from")
	quality     = flag.String("quality", "", "Preferred video quality (e.g. 720p, 480p, 360p, 240p); defaults to best available")
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage of %s:\n", appName)
		fmt.Printf("  grabber <playlist_url> <output.mpg>\n")
		fmt.Printf("  grabber -yadisk=<share_url> [output.mpg]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	videoUrl := flag.Arg(0)
	outputName := flag.Arg(1)

	if outputName == "" {
		outputName = defaultOutputFileName
	}

	// Resolve Yandex Disk share page to an M3U8 playlist URL.
	if *yadiskURL != "" {
		// When -yadisk is used, the optional positional arg is the output filename.
		if outputName == defaultOutputFileName && flag.Arg(0) != "" && flag.Arg(1) == "" {
			outputName = flag.Arg(0)
		}
		resolved, err := resolveYadisk(*yadiskURL, *quality)
		if err != nil {
			log.Fatalf("[!] %v", err)
		}
		videoUrl = resolved
	}

	if videoUrl == "" && (*playlist == "") {
		flag.Usage()
		os.Exit(0)
	}

	appInstance := app.NewApp(videoUrl, *threads, *verboseMode, playlist, outputName)
	appInstance.Run()
}

func resolveYadisk(pageURL, preferredQuality string) (string, error) {
	fmt.Printf("[+] Fetching Yandex Disk page: %s\n", pageURL)
	streams, err := yadisk.FetchStreams(pageURL)
	if err != nil {
		return "", fmt.Errorf("yadisk: %w", err)
	}

	stream, err := yadisk.SelectBest(streams, preferredQuality)
	if err != nil {
		return "", fmt.Errorf("yadisk: %w", err)
	}

	fmt.Printf("[+] Selected quality: %s\n", stream.Dimension)
	return stream.URL, nil
}

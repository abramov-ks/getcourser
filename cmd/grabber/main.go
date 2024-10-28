package main

import (
	"flag"
	"fmt"
	"getcourser/internal/app"
	"os"
)

const appName = "get-course-grabber"
const defaultOutputFileName = "video.mpg"

var (
	threads     = flag.Int("threads", 5, "Number of threads to use")
	verboseMode = flag.Bool("verbose", false, "Verbose mode")
	videoUrl    = flag.Arg(0)
	outputName  = flag.Arg(1)
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage of %s: grabber <playlist_url> <output.mpg>\n", appName)
		flag.PrintDefaults()
	}
	flag.Parse()

	if outputName == "" {
		outputName = defaultOutputFileName
	}

	if videoUrl == "" {
		flag.Usage()
		os.Exit(0)
	}

	appInstance := app.NewApp(videoUrl, *threads, *verboseMode, outputName)
	appInstance.Run()
}

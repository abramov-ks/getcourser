package m3u

import (
	"bufio"
	"log"
	"os"
	"strings"
)

type M3u struct {
	filePath string
}

func (u *M3u) GetSlices() ([]string, error) {
	slices := make([]string, 0)
	file, err := os.Open(u.filePath)
	if err != nil {
		return slices, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "#EXTINF") {
			if scanner.Scan() {
				slices = append(slices, scanner.Text())
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return slices, nil
}

func NewM3u(filePath string) *M3u {
	return &M3u{filePath: filePath}
}

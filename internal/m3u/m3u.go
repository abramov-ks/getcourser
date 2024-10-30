package m3u

import (
	"bufio"
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
				currentSlice := scanner.Text()
				if len(strings.TrimSpace(currentSlice)) > 0 {
					slices = append(slices, currentSlice)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return slices, nil
}

func New(filePath string) *M3u {
	return &M3u{filePath: filePath}
}

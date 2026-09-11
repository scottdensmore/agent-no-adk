package tools

import (
	"bufio"
	"fmt"
	"os"
)

type ReadLogArgs struct {
	FilePath string `json:"file_path"`
	MaxLines int    `json:"max_lines,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

type ReadLogResult struct {
	FilePath   string   `json:"file_path"`
	TotalLines int      `json:"total_lines"`
	Offset     int      `json:"offset"`
	LinesRead  int      `json:"lines_read"`
	Lines      []string `json:"lines"`
}

func ReadLogFile(path string, maxLines int, offset int) (*ReadLogResult, error) {
	if maxLines <= 0 {
		maxLines = 200
	}
	if maxLines > 1000 {
		maxLines = 1000
	}
	if offset < 0 {
		offset = 0
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file %q: %w", path, err)
	}

	total := len(allLines)
	if offset >= total {
		return &ReadLogResult{
			FilePath:   path,
			TotalLines: total,
			Offset:     offset,
			LinesRead:  0,
			Lines:      []string{},
		}, nil
	}

	end := offset + maxLines
	if end > total {
		end = total
	}

	sliced := allLines[offset:end]
	return &ReadLogResult{
		FilePath:   path,
		TotalLines: total,
		Offset:     offset,
		LinesRead:  len(sliced),
		Lines:      sliced,
	}, nil
}

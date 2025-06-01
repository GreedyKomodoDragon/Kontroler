package logs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// fsLogFetcher implements LogFetcher interface for filesystem-based log storage
type fsLogFetcher struct {
	basePath string
}

// NewFSLogFetcher creates a new filesystem-based log fetcher
// basePath is the root directory where logs are stored
func NewFSLogFetcher(basePath string) (LogFetcher, error) {
	// Check if the base path exists
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("log directory does not exist: %s", basePath)
		}
		return nil, fmt.Errorf("error checking log directory: %v", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", basePath)
	}

	return &fsLogFetcher{
		basePath: basePath,
	}, nil
}

// RangeFetchLogs implements LogFetcher.RangeFetchLogs for filesystem
func (f *fsLogFetcher) RangeFetchLogs(logKey *string, start int64, end int64) (io.ReadCloser, error) {
	if logKey == nil {
		return nil, fmt.Errorf("logKey cannot be nil")
	}

	fullPath := filepath.Join(f.basePath, *logKey)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}

	// Seek to the start position
	if _, err := file.Seek(start, 0); err != nil {
		file.Close()
		return nil, err
	}

	// Create a LimitReader to handle the range
	rangeSize := end - start + 1
	return &rangeReadCloser{
		reader: io.LimitReader(file, rangeSize),
		closer: file,
	}, nil
}

// FetchLogs implements LogFetcher.FetchLogs for filesystem
func (f *fsLogFetcher) FetchLogs(logKey *string) (io.ReadCloser, error) {
	if logKey == nil {
		return nil, fmt.Errorf("logKey cannot be nil")
	}

	fullPath := filepath.Join(f.basePath, *logKey)
	return os.Open(fullPath)
}

// LogFileExists implements LogFetcher.LogFileExists for filesystem
func (f *fsLogFetcher) LogFileExists(logFileKey *string) (bool, int64, error) {
	if logFileKey == nil {
		return false, 0, fmt.Errorf("logFileKey cannot be nil")
	}

	fullPath := filepath.Join(f.basePath, *logFileKey)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	return true, info.Size(), nil
}

// rangeReadCloser combines a Reader with a Closer
type rangeReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (r *rangeReadCloser) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *rangeReadCloser) Close() error {
	return r.closer.Close()
}

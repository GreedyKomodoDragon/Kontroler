package object

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type fileSystemLogStore struct {
	baseDir  string
	fetching map[string]bool
	lock     *sync.RWMutex
}

func NewFileSystemLogStore(baseDir string) (LogStore, error) {
	if baseDir == "" {
		baseDir = "/tmp/kontroler-logs"
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	return &fileSystemLogStore{
		baseDir:  baseDir,
		fetching: make(map[string]bool),
		lock:     &sync.RWMutex{},
	}, nil
}

func (f *fileSystemLogStore) IsFetching(dagRunId int, pod *v1.Pod) bool {
	f.lock.RLock()
	defer f.lock.RUnlock()

	_, ok := f.fetching[fmt.Sprintf("%v-%s", dagRunId, pod.Name)]
	return ok
}

func (f *fileSystemLogStore) MarkAsFetching(dagRunId int, pod *v1.Pod) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	key := fmt.Sprintf("%v-%s", dagRunId, pod.Name)
	if _, ok := f.fetching[key]; ok {
		return fmt.Errorf("already fetching")
	}

	f.fetching[key] = true
	return nil
}

func (f *fileSystemLogStore) UnlistFetching(dagRunId int, pod *v1.Pod) {
	f.lock.Lock()
	defer f.lock.Unlock()

	delete(f.fetching, fmt.Sprintf("%v-%s", dagRunId, pod.Name))
}

func (f *fileSystemLogStore) UploadLogs(ctx context.Context, dagrunId int, clientSet *kubernetes.Clientset, pod *v1.Pod) error {
	defer func() {
		if err := RemoveFinalizer(clientSet, pod.Name, pod.Namespace, "kontroler/logcollection"); err != nil {
			log.Log.Error(err, "error removing finalizer", "pod", pod.Name, "namespace", pod.Namespace)
		}
	}()

	logDir := filepath.Join(f.baseDir, fmt.Sprintf("%d", dagrunId))
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("%s-log.txt", pod.UID))
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	req := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		Follow: true,
	})

	logStream, err := req.Stream(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Log.Info("pod already deleted, cannot fetch logs", "pod", pod.Name)
			return nil
		}
		return fmt.Errorf("error in opening stream: %v", err)
	}
	defer logStream.Close()

	reader := bufio.NewReader(logStream)
	writer := bufio.NewWriter(logFile)

	buffer := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			if _, err := writer.Write(buffer[:n]); err != nil {
				return fmt.Errorf("error writing to log file: %v", err)
			}
			if err := writer.Flush(); err != nil {
				return fmt.Errorf("error flushing log file: %v", err)
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			if strings.Contains(readErr.Error(), "not found") ||
				strings.Contains(readErr.Error(), "connection refused") ||
				strings.Contains(readErr.Error(), "has been terminated") {
				log.Log.Info("pod deleted while reading logs", "pod", pod.Name)
				return nil
			}
			return fmt.Errorf("error reading logs: %v", readErr)
		}

		time.Sleep(time.Second)
	}

	log.Log.Info("Logs successfully written to file", "path", logPath)
	return nil
}

func (f *fileSystemLogStore) DeleteLogs(ctx context.Context, dagrunId int) error {
	logDir := filepath.Join(f.baseDir, fmt.Sprintf("%d", dagrunId))
	if err := os.RemoveAll(logDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete log directory: %v", err)
	}

	log.Log.Info("Successfully deleted all logs", "directory", logDir)
	return nil
}

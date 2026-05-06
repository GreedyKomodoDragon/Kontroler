package object

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type fileSystemLogStore struct {
	baseDir  string
	fetching map[string]bool
	lock     *sync.RWMutex

	// configurable for tests
	streamRetryCount  int
	streamOpenTimeout time.Duration
}

func NewFileSystemLogStore(baseDir string) (LogStore, error) {
	if baseDir == "" {
		baseDir = "/tmp/kontroler-logs"
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
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

	// Use pod UID to avoid collisions when pod names are reused
	_, ok := f.fetching[fmt.Sprintf("%v-%s", dagRunId, pod.UID)]
	return ok
}

func (f *fileSystemLogStore) MarkAsFetching(dagRunId int, pod *v1.Pod) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	// Use pod UID to avoid collisions when pod names are reused
	key := fmt.Sprintf("%v-%s", dagRunId, pod.UID)
	if _, ok := f.fetching[key]; ok {
		return fmt.Errorf("already fetching")
	}

	f.fetching[key] = true
	return nil
}

func (f *fileSystemLogStore) UnlistFetching(dagRunId int, pod *v1.Pod) {
	f.lock.Lock()
	defer f.lock.Unlock()

	delete(f.fetching, fmt.Sprintf("%v-%s", dagRunId, pod.UID))
}

type podLogsGetter interface {
	GetLogs(podName string, opts *v1.PodLogOptions) podLogStreamer
}

type podLogStreamer interface {
	Stream(ctx context.Context) (io.ReadCloser, error)
}

type coreV1PodLogsGetter struct {
	client *kubernetes.Clientset
	ns     string
}

func (c *coreV1PodLogsGetter) GetLogs(podName string, opts *v1.PodLogOptions) podLogStreamer {
	return c.client.CoreV1().Pods(c.ns).GetLogs(podName, opts)
}

func (f *fileSystemLogStore) UploadLogs(ctx context.Context, dagrunId int, clientSet *kubernetes.Clientset, pod *v1.Pod) error {
	getter := &coreV1PodLogsGetter{client: clientSet, ns: pod.Namespace}
	return f.uploadLogsWithGetter(ctx, dagrunId, getter, pod, func() {
		// remove finaliser when done
		if err := RemoveFinalizer(clientSet, pod.Name, pod.Namespace, "kontroler/logcollection"); err != nil {
			log.Log.Error(err, "error removing finalizer", "pod", pod.Name, "namespace", pod.Namespace)
		}
	})
}

func (f *fileSystemLogStore) uploadLogsWithGetter(ctx context.Context, dagrunId int, getter podLogsGetter, pod *v1.Pod, finaliserCleanup func()) error {
	shouldCleanup := false
	defer func() {
		if shouldCleanup && finaliserCleanup != nil {
			finaliserCleanup()
		}
	}()

	logDir := filepath.Join(f.baseDir, fmt.Sprintf("%d", dagrunId))
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("%s-log.txt", pod.UID))
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	var logStream io.ReadCloser
	var lastErr error

	retryCount := f.streamRetryCount
	if retryCount <= 0 {
		retryCount = defaultStreamRetryCount
	}

	if len(pod.Spec.Containers) == 0 {
		err := fmt.Errorf("no containers configured for pod")
		log.Log.Error(err, "cannot fetch logs", "pod", pod.Name)
		return err
	}

	for i := 0; i < retryCount; i++ {
		req := getter.GetLogs(pod.Name, &v1.PodLogOptions{
			Follow:    true,
			Container: pod.Spec.Containers[0].Name,
		})

		logStream, lastErr = req.Stream(ctx)
		if lastErr == nil {
			break
		}

		if apierrors.IsNotFound(lastErr) || strings.Contains(lastErr.Error(), "not found") {
			log.Log.Info("pod already deleted, cannot fetch logs", "pod", pod.Name, "error", lastErr)
			shouldCleanup = true
			return nil
		}

		if i < retryCount-1 {
			backoff := time.Duration(1<<i) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}

		return fmt.Errorf("error in opening stream: %w", lastErr)
	}

	if logStream == nil {
		return fmt.Errorf("failed to open log stream: %v", lastErr)
	}
	defer logStream.Close()

	writer := bufio.NewWriter(logFile)
	if _, err := io.Copy(writer, logStream); err != nil {
		if apierrors.IsNotFound(err) {
			log.Log.Info("pod deleted while reading logs", "pod", pod.Name, "error", err)
			shouldCleanup = true
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			log.Log.Info("log stream ended", "pod", pod.Name, "error", err)
		} else if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "has been terminated") {
			log.Log.Info("pod deleted while reading logs", "pod", pod.Name, "error", err)
			shouldCleanup = true
			return nil
		} else {
			return fmt.Errorf("error reading logs: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("error flushing log file: %w", err)
	}

	shouldCleanup = true
	log.Log.Info("Logs successfully written to file", "path", logPath)
	return nil
}

func (f *fileSystemLogStore) DeleteLogs(ctx context.Context, dagrunId int) error {
	logDir := filepath.Join(f.baseDir, fmt.Sprintf("%d", dagrunId))
	if err := os.RemoveAll(logDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete log directory: %w", err)
	}

	log.Log.Info("Successfully deleted all logs", "directory", logDir)
	return nil
}

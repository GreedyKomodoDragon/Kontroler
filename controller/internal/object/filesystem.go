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
	defer func() {
		if finaliserCleanup != nil {
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

	// Attempt to open the log stream with a short timeout and a configurable number of retries.
	var logStream io.ReadCloser
	var cancelStream context.CancelFunc
	var lastErr error

	retryCount := f.streamRetryCount
	topen := f.streamOpenTimeout
	if retryCount == 0 {
		retryCount = defaultStreamRetryCount
	}
	if topen == 0 {
		topen = defaultStreamOpenTimeout
	}

	for i := 0; i < retryCount; i++ {
		openCtx, cancel := context.WithTimeout(ctx, topen)
		// guard container presence
		if len(pod.Spec.Containers) == 0 {
			log.Log.Error(fmt.Errorf("no containers in pod"), "cannot fetch logs", "pod", pod.Name)
			cancel()
			return nil
		}
		req := getter.GetLogs(pod.Name, &v1.PodLogOptions{
			Follow:    true,
			Container: pod.Spec.Containers[0].Name,
		})

		logStream, lastErr = req.Stream(openCtx)
		if lastErr == nil {
			cancelStream = cancel
			break
		}

		// clean up this attempt's context
		cancel()

		if strings.Contains(lastErr.Error(), "not found") {
			log.Log.Info("pod already deleted, cannot fetch logs", "pod", pod.Name)
			return nil
		}

		// transient error - retry
		if i < retryCount-1 {
			time.Sleep(time.Duration(1<<i) * time.Second)
			continue
		}

		return fmt.Errorf("error in opening stream: %w", lastErr)
	}

	if logStream == nil {
		return fmt.Errorf("failed to open log stream: %v", lastErr)
	}
	defer func() {
		if cancelStream != nil {
			cancelStream()
		}
	}()
	defer logStream.Close()

	writer := bufio.NewWriter(logFile)
	// Use io.Copy which is simpler and efficient for streaming until EOF
	if _, err := io.Copy(writer, logStream); err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "has been terminated") {
			log.Log.Info("pod deleted while reading logs", "pod", pod.Name)
			return nil
		}
		return fmt.Errorf("error reading logs: %w", err)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("error flushing log file: %w", err)
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
		return fmt.Errorf("failed to delete log directory: %w", err)
	}

	log.Log.Info("Successfully deleted all logs", "directory", logDir)
	return nil
}

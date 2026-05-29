package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/metrics"
	"kontroler-controller/internal/object"
	"kontroler-controller/internal/queue"
	"kontroler-controller/internal/webhook"
	"kontroler-controller/internal/workers/container"

	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// Constants for error messages and annotations
const (
	kontrolerTaskID      = "kontroler/task-id"
	kontrolerDagRunID    = "kontroler/dagRun-id"
	errMsgDagRunID       = "failed to get dag run ID"
	errMsgWebhookDetails = "failed to get webhook details"
	errMsgTaskRunID      = "failed to get task run ID"
	errFormatWithString  = "%w: %s"
)

type Worker[T any] interface {
	Push(item T, event string) error
	Run(ctx context.Context) error
	Queue() queue.Queue
	ID() string
}

type worker struct {
	queue           queue.Queue
	dbManager       db.DBDAGManager
	clientSet       *kubernetes.Clientset
	taskAllocator   TaskAllocator
	logStore        object.LogStore
	webhookNotifier webhook.WebhookNotifier
	id              string
	pollDuration    time.Duration
	inFlight        int32
}

func NewWorker(queue queue.Queue, logStore object.LogStore, webhookChan chan webhook.WebhookPayload,
	dbManager db.DBDAGManager, clientSet *kubernetes.Clientset, taskAllocator TaskAllocator,
	pollDuration time.Duration) Worker[*v1.Pod] {
	return &worker{
		queue:           queue,
		logStore:        logStore,
		webhookNotifier: webhook.NewWebhookNotifier(webhookChan),
		dbManager:       dbManager,
		clientSet:       clientSet,
		taskAllocator:   taskAllocator,
		id:              uuid.NewString(),
		pollDuration:    pollDuration,
	}
}

func (w *worker) ID() string {
	return w.id
}

func (w *worker) Queue() queue.Queue {
	return w.queue
}

func (w *worker) Push(pod *v1.Pod, event string) error {
	return w.queue.Push(&queue.PodEvent{
		Pod:   pod,
		Event: event,
	})
}

// Claiming defaults
var (
	// defaultLeaseTTL is the lease duration for claimed tasks. Made a var to allow
	// tests to override this value for faster unit tests.
	defaultLeaseTTL = 60 * time.Second
	claimBatchSize  = 5
)

func (w *worker) claimPoller(ctx context.Context) {
	ticker := time.NewTicker(w.pollDuration)
	defer ticker.Stop()

	const maxConcurrentClaims = 50

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// limit concurrent claim processing to avoid unbounded goroutine growth
			inFlight := atomic.LoadInt32(&w.inFlight)
			allowed := maxConcurrentClaims - inFlight
			if allowed <= 0 {
				// nothing to do this tick
				continue
			}

			limit := claimBatchSize
			if int32(limit) > allowed {
				limit = int(allowed)
			}

			claims, err := w.dbManager.ClaimTasks(ctx, limit, w.id, defaultLeaseTTL)
			if err != nil {
				log.Log.Error(err, "failed to claim tasks")
				metrics.RecordTaskClaim(w.id, "error")
				continue
			}

			if len(claims) == 0 {
				metrics.RecordTaskClaim(w.id, "none")
				continue
			}

			for _, c := range claims {
				c := c
				metrics.RecordTaskClaim(w.id, "success")
				metrics.IncClaimed(w.id)
				atomic.AddInt32(&w.inFlight, 1)
				go func(cl db.TaskClaim) {
					defer metrics.DecClaimed(w.id)
					defer atomic.AddInt32(&w.inFlight, -1)
					w.processClaim(ctx, cl)
				}(c)
			}
		}
	}
}

// processClaim handles a single claimed task: renew lease, allocate pod, and finalize claim.
func (w *worker) processClaim(ctx context.Context, c db.TaskClaim) {
	log.Log.Info("processing claimed task", "taskRunId", c.TaskRunID, "taskId", c.TaskID, "runId", c.RunID, "worker", w.id)

	// lease renew context
	renewCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// start lease renew goroutine
	go func() {
		ticker := time.NewTicker(defaultLeaseTTL / 2)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := w.dbManager.RenewLease(renewCtx, c.TaskRunID, w.id, defaultLeaseTTL); err != nil {
					log.Log.Error(err, "failed to renew lease", "taskRunId", c.TaskRunID)
					metrics.RecordLeaseRenew(w.id, "error")
					// abort allocation/finalization on lease renewal failure
					cancel()
					return
				}
				metrics.RecordLeaseRenew(w.id, "success")
				// log a small info record so we can observe renewals in logs during testing
				log.Log.Info("lease renewed", "taskRunId", c.TaskRunID, "worker", w.id)
			}
		}
	}()

	// fetch task details and namespace plus retry env
	task, namespace, retryEnv, err := w.dbManager.GetTaskForRun(ctx, c.RunID, c.TaskID)
	if err != nil {
		log.Log.Error(err, "failed to get task for run", "runId", c.RunID, "taskId", c.TaskID)
		return
	}

	// Prepare envs: if retryEnv provided, use it; otherwise let allocator create envs
	var podUID types.UID
	if retryEnv != "" {
		// parse retryEnv JSON into []v1.EnvVar to preserve ValueFrom fields
		var envs []v1.EnvVar
		if err := json.Unmarshal([]byte(retryEnv), &envs); err != nil {
			log.Log.Error(err, "failed to parse retry env JSON", "taskRunId", c.TaskRunID)
			// fall back to normal allocation
			podUID, err = w.taskAllocator.AllocateTask(ctx, &task, c.RunID, c.TaskRunID, namespace, w.id)
			if err != nil {
				log.Log.Error(err, "failed to allocate pod for claimed task", "taskRunId", c.TaskRunID)
				return
			}
		} else {
			podUID, err = w.taskAllocator.AllocateTaskWithEnv(ctx, &task, c.RunID, c.TaskRunID, namespace, envs, nil, w.id)
			if err != nil {
				log.Log.Error(err, "failed to allocate pod for claimed task with retry env", "taskRunId", c.TaskRunID)
				return
			}
		}
	} else {
		// normal allocation path
		podUID, err = w.taskAllocator.AllocateTask(ctx, &task, c.RunID, c.TaskRunID, namespace, w.id)
		if err != nil {
			log.Log.Error(err, "failed to allocate pod for claimed task", "taskRunId", c.TaskRunID)
			// leave claim to expire or be recovered
			return
		}
	}

	// finalize claim: set status to running
	if err := w.dbManager.FinalizeClaimToRunning(ctx, c.TaskRunID, w.id, string(podUID)); err != nil {
		log.Log.Error(err, "failed to finalize claim to running", "taskRunId", c.TaskRunID)
		// best-effort: try to delete the created pod to avoid orphaned pods if we can find it
		if w.clientSet != nil && podUID != "" {
			// List pods by UID to find the pod name
			pods, listErr := w.clientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{FieldSelector: "metadata.uid=" + string(podUID)})
			if listErr == nil && len(pods.Items) > 0 {
				_ = w.clientSet.CoreV1().Pods(namespace).Delete(ctx, pods.Items[0].Name, metav1.DeleteOptions{})
			}
		}
		return
	}

	log.Log.Info("claim finalized and pod created", "taskRunId", c.TaskRunID, "podUID", podUID)
}

func (w *worker) Run(ctx context.Context) error {
	log.Log.Info("worker started")

	// start claim poller
	go w.claimPoller(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		podEvent, err := w.queue.PopWithContext(ctx)
		if err != nil {
			if errors.Is(err, queue.ErrQueueIsEmpty) {
				// queue closed or empty; loop and check ctx
				continue
			}

			log.Log.Error(err, "failed to pop pod event from queue")
			continue
		}

		switch podEvent.Event {
		case "add":
			log.Log.Info("pod was added", "podUID", podEvent.Pod.UID, "name", podEvent.Pod.Name)
			w.handleAdd(ctx, podEvent.Pod, podEvent.EventTime)
		case "update":
			log.Log.Info("pod was updated", "podUID", podEvent.Pod.UID, "name", podEvent.Pod.Name)
			w.handleUpdate(ctx, podEvent.Pod, podEvent.EventTime)
		default:
			log.Log.Info("unknown event", "event", podEvent.Event)
		}
	}
}

func (w *worker) handleAdd(ctx context.Context, pod *v1.Pod, eventTime *time.Time) {
	// Record worker processing metric
	metrics.RecordWorkerTaskProcessing(w.id, "add")
	w.handleOutcome(ctx, pod, "add", eventTime)
}

func (t *worker) handleUpdate(ctx context.Context, pod *v1.Pod, eventTime *time.Time) {
	// Record worker processing metric
	metrics.RecordWorkerTaskProcessing(t.id, "update")
	t.handleOutcome(ctx, pod, "update", eventTime)
}

func (w *worker) handleOutcome(ctx context.Context, pod *v1.Pod, event string, eventTime *time.Time) {
	log.Log.Info("pod event", "worker", w.id, "podUID", pod.UID, "name", pod.Name, "event", event, "eventTime", eventTime)

	// Update queue size metric
	if queueSize, err := w.queue.Size(); err == nil {
		metrics.UpdateWorkerQueueSize(w.id, int(queueSize))
	}

	taskRunId, err := w.getTaskRunID(pod)
	if err != nil {
		log.Log.Error(err, errMsgTaskRunID, "pod", pod.Name)
		return
	}

	w.handleLogCollection(ctx, pod)

	writeState := true

	switch pod.Status.Phase {
	case v1.PodSucceeded:
		w.handleSuccessfulTaskRun(ctx, pod, taskRunId)
	case v1.PodFailed:
		w.handleFailedTaskRun(ctx, pod, taskRunId)
	case v1.PodRunning:
		w.handleStartedTaskRun(ctx, pod, taskRunId)
	case v1.PodPending:
		// there is a special case if config error is detected
		// in that case we treat it as a failure
		// and do not write to db as handleConfigError will do that
		writeState = w.handlePendingTaskRun(ctx, pod, taskRunId)
	case v1.PodUnknown:
		log.Log.Info("pod status unknown", "podUID", pod.UID, "name", pod.Name, "event", event)
	}

	if writeState {
		if err := w.writeStatusToDB(ctx, pod, eventTime); err != nil {
			log.Log.Error(err, "failed to writeStatusToDB")
		}
	}
}

func (w *worker) handleSuccessfulTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task succeeded", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	w.recordSuccessMetrics(ctx, pod, taskRunId)

	dagRunId, err := w.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, errMsgDagRunID, "pod", pod.Name)
		return
	}

	if err := w.deletePod(ctx, pod, false); err != nil {
		log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		return
	}

	tasks, err := w.dbManager.MarkSuccessAndGetNextTasks(ctx, taskRunId)
	if err != nil {
		log.Log.Error(err, "failed to mark outcome and get next task", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
		return
	}

	w.sendSuccessWebhook(ctx, pod, dagRunId, taskRunId)

	w.processNextTasks(ctx, pod, dagRunId, tasks)
}

func (w *worker) recordSuccessMetrics(ctx context.Context, pod *v1.Pod, taskRunId int) {
	// Get DAG and task names for metrics
	dagName, taskName, namespace := w.getTaskRunMetricsInfo(ctx, taskRunId)

	// Record task outcome metric
	metrics.RecordTaskOutcome(namespace, dagName, taskName, "success")

	// Record task execution duration if available
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		terminated := pod.Status.ContainerStatuses[0].State.Terminated
		duration := terminated.FinishedAt.Sub(terminated.StartedAt.Time).Seconds()
		metrics.RecordTaskExecutionDuration(namespace, dagName, taskName, "success", duration)
	}
}

func (w *worker) sendSuccessWebhook(ctx context.Context, pod *v1.Pod, dagRunId, taskRunId int) {
	webhook, err := w.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, errMsgWebhookDetails, "runId", dagRunId)
	} else if webhook.URL != "" {
		go w.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "success", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}
}

func (w *worker) processNextTasks(ctx context.Context, pod *v1.Pod, dagRunId int, tasks []db.Task) {
	log.Log.Info("number of tasks", "tasks", len(tasks))

	if len(tasks) == 0 {
		w.handleDagRunCompletion(ctx, pod, dagRunId)
		return
	}

	w.allocateNextTasks(ctx, pod, dagRunId, tasks)
}

func (w *worker) handleDagRunCompletion(ctx context.Context, pod *v1.Pod, dagRunId int) {
	complete, err := w.checkIfDagRunIsComplete(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to check if dag run is complete", "runId", dagRunId)
		return
	}

	if !complete {
		return
	}

	if err := w.deletePVC(ctx, pod); err != nil {
		log.Log.Error(err, "failed to delete PVC", "pod", pod.Name, "namespace", pod.Namespace, "dagRunId", dagRunId, "status", pod.Status.Phase)
	}
}

func (w *worker) allocateNextTasks(ctx context.Context, pod *v1.Pod, dagRunId int, tasks []db.Task) {
	for _, task := range tasks {
		// create pending task run for workers to claim
		taskRunId, err := w.dbManager.AddPendingTaskRun(ctx, dagRunId, task.Id)
		if err != nil {
			log.Log.Error(err, "failed to add pending task run", "dagRun_id", dagRunId, "task_id", task.Id)
			continue
		}

		log.Log.Info("enqueued pending task", "taskRunId", taskRunId, "task.Id", task.Id, "task.Name", task.Name)
	}
}

func (t *worker) handleFailedTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	// Use computePodDurationAndExit to safely obtain exit code without
	// dereferencing Terminated when it may be nil.
	_, _, exitPtr := t.computePodDurationAndExit(pod, nil)
	var exitcode int32 = -1
	if exitPtr != nil {
		exitcode = *exitPtr
	}

	t.recordFailureMetrics(ctx, pod, taskRunId)

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, errMsgDagRunID, "pod", pod.Name)
		return
	}

	if err := t.deletePod(ctx, pod, false); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		} else {
			log.Log.Error(err, "failed to delete pod in failed,", "podUId", pod.UID)
			return
		}
	}

	// For pods that failed before container start, always attempt a retry
	ok, err := t.dbManager.ShouldRerun(ctx, taskRunId, exitcode)
	if err != nil {
		log.Log.Error(err, "failed to determine if pod should be re-ran", "pod", pod.Name)
		return
	}

	if !ok {
		t.handleUnretryablePod(ctx, pod, taskRunId, dagRunId, exitcode)
		return
	}

	t.retryFailedTask(ctx, pod, dagRunId, taskRunId, exitcode)
}

func (t *worker) getExitCode(pod *v1.Pod, taskRunId int) int32 {
	// mark exitcode as -1 if pod was deleted before it started - means something odd happened
	var exitcode int32 = -1
	if len(pod.Status.ContainerStatuses) == 0 {
		log.Log.Info("task failed without container status", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)
	} else {
		log.Log.Info("task failed", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "exitcode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
		exitcode = pod.Status.ContainerStatuses[0].State.Terminated.ExitCode
	}
	return exitcode
}

func (t *worker) recordFailureMetrics(ctx context.Context, pod *v1.Pod, taskRunId int) {
	// Get DAG and task names for metrics
	dagName, taskName, namespace := t.getTaskRunMetricsInfo(ctx, taskRunId)

	// Record task outcome metric
	metrics.RecordTaskOutcome(namespace, dagName, taskName, "failed")

	// Record task execution duration if available
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		terminated := pod.Status.ContainerStatuses[0].State.Terminated
		duration := terminated.FinishedAt.Sub(terminated.StartedAt.Time).Seconds()
		metrics.RecordTaskExecutionDuration(namespace, dagName, taskName, "failed", duration)
	}
}

func (t *worker) retryFailedTask(ctx context.Context, pod *v1.Pod, dagRunId, taskRunId int, exitcode int32) {
	// Get DAG and task names for metrics
	dagName, taskName, namespace := t.getTaskRunMetricsInfo(ctx, taskRunId)

	// Record retry metric
	metrics.RecordTaskRetry(namespace, dagName, taskName, fmt.Sprintf("exit_code_%d", exitcode))

	// Create a new pending task run and save retry env
	taskId, err := t.getTaskIdFromPod(pod)
	if err != nil {
		return
	}

	newTaskRunId, err := t.dbManager.AddPendingTaskRun(ctx, dagRunId, taskId)
	if err != nil {
		log.Log.Error(err, "failed to create pending task run for retry")
		return
	}

	// Save retry env (serialize container env to JSON)
	container := pod.Spec.Containers[0]
	envs := make([]map[string]string, 0, len(container.Env))
	for _, e := range container.Env {
		envs = append(envs, map[string]string{"name": e.Name, "value": e.Value})
	}
	b, _ := json.Marshal(envs)
	if err := t.dbManager.SaveRetryEnv(ctx, newTaskRunId, string(b)); err != nil {
		log.Log.Error(err, "failed to save retry env")
	}

	// Claim the new task immediately
	claim, err := t.dbManager.ClaimTaskByID(ctx, newTaskRunId, t.id, defaultLeaseTTL)
	if err != nil {
		log.Log.Error(err, "failed to claim retry task")
		return
	}

	// Process the claimed task (this will allocate pod and finalize)
	go t.processClaim(ctx, claim)

	if err := t.dbManager.IncrementAttempts(ctx, newTaskRunId); err != nil {
		log.Log.Error(err, "failed to increment attempts", "taskRunId", newTaskRunId)
	}

	log.Log.Info("retry task created and claimed", "newTaskRunId", newTaskRunId)
}

func (t *worker) getTaskIdFromPod(pod *v1.Pod) (int, error) {
	taskIdStr, ok := pod.Annotations[kontrolerTaskID]
	if !ok {
		log.Log.Error(fmt.Errorf("missing annotation"), "annotation", kontrolerTaskID, "pod", pod.Name)
		return 0, fmt.Errorf("missing annotation")
	}

	taskId, err := strconv.Atoi(taskIdStr)
	if err != nil {
		log.Log.Error(fmt.Errorf("failed to convert task id string: %s", taskIdStr), "annotation", kontrolerTaskID, "value", taskIdStr, "pod", pod.Name)
		return 0, err
	}

	return taskId, nil
}

func (t *worker) createTaskFromPod(ctx context.Context, pod *v1.Pod, taskId int) (*db.Task, error) {
	script, injector, err := t.dbManager.GetTaskScriptAndInjectorImage(ctx, taskId)
	if err != nil {
		log.Log.Error(err, "GetTaskScriptAndInjectorImage failed", "pod", pod.Name)
		return nil, err
	}

	container := pod.Spec.Containers[0]
	dbTask := &db.Task{
		Id:          taskId,
		Name:        container.Name,
		Args:        container.Args,
		Command:     container.Command,
		Image:       container.Image,
		PodTemplate: v1alpha1.PodTemplateSpecFromK8s(&pod.Spec, &container),
	}

	if script != nil {
		dbTask.Script = *script
	}

	if injector != nil {
		dbTask.ScriptInjectorImage = *injector
	}

	return dbTask, nil
}

// computePodDurationAndExit centralizes logic that determines the pod exit code, a
// best-effort duration (seconds) and the timestamp that should be treated as the
// "finished" time for the pod event. This keeps the heuristics in one place so
// other code paths (metrics, DB writes, webhooks) are consistent.
func (w *worker) computePodDurationAndExit(pod *v1.Pod, eventStamp *time.Time) (duration int64, stamp *time.Time, exitCode *int32) {
	stamp = eventStamp
	var dur int64 = 0
	var exit *int32 = nil

	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		terminated := pod.Status.ContainerStatuses[0].State.Terminated
		exit = &terminated.ExitCode

		// Prefer the explicit termination times if present
		if !terminated.FinishedAt.IsZero() {
			stamp = &terminated.FinishedAt.Time
		}

		// Compute duration defensively. Some kubelets may not set StartedAt/FinishedAt
		// in every failure mode, so avoid using zero-times which lead to huge durations.
		var d int64 = 0
		if !terminated.StartedAt.IsZero() && !terminated.FinishedAt.IsZero() {
			d = int64(terminated.FinishedAt.Sub(terminated.StartedAt.Time).Seconds())
		} else if !terminated.StartedAt.IsZero() && terminated.FinishedAt.IsZero() {
			// If finished missing, estimate using now
			d = int64(time.Since(terminated.StartedAt.Time).Seconds())
		} else if terminated.StartedAt.IsZero() && !terminated.FinishedAt.IsZero() {
			// If started missing, try to fall back to pod.Status.StartTime
			if pod.Status.StartTime != nil && !pod.Status.StartTime.IsZero() {
				d = int64(terminated.FinishedAt.Sub(pod.Status.StartTime.Time).Seconds())
			} else {
				// Unknown start; set to 0 to avoid absurd values
				d = 0
			}
		} else {
			// Both timestamps missing — set duration to 0
			d = 0
		}

		if d < 0 {
			d = 0
		}
		dur = d
	} else if pod.Status.Phase == v1.PodFailed {
		// Handle case where pod failed before container started
		defaultExitCode := int32(-1)
		exit = &defaultExitCode
	}

	return dur, stamp, exit
}

func (w *worker) writeStatusToDB(ctx context.Context, pod *v1.Pod, stamp *time.Time) error {
	taskRunId, err := w.getTaskRunID(pod)
	if err != nil {
		return err
	}

	dagRunId, err := w.getDagRunID(pod)
	if err != nil {
		return err
	}

	// Consolidated computation for duration/exit/timestamp
	duration, outStamp, exitCode := w.computePodDurationAndExit(pod, stamp)

	// Only add durations when we have a non-zero computed value. Historical bad
	// values are sanitized on read; this prevents writing absurd values going
	// forward.
	if duration > 0 {
		if err := w.dbManager.AddPodDuration(ctx, taskRunId, duration); err != nil {
			log.Log.Error(err, "failed to add pod duration", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)
		}
	}

	if outStamp == nil {
		now := time.Now()
		outStamp = &now
	}

	// Try to write pod status with retries on transient 'pod status not updated' races.
	const maxStatusRetries = 5
	var lastErr error
	for i := 0; i < maxStatusRetries; i++ {
		if err := w.dbManager.MarkPodStatus(ctx, pod.UID, pod.Name,
			taskRunId, pod.Status.Phase, *outStamp, exitCode, pod.Namespace); err != nil {
			lastErr = err
			// Treat the specific DB condition as transient and retry with backoff
			if strings.Contains(err.Error(), "pod status not updated") {
				backoff := time.Duration(100*(1<<i)) * time.Millisecond
				log.Log.Info("pod status write conflicted, retrying", "pod", pod.Name, "attempt", i+1, "backoff", backoff)
				time.Sleep(backoff)
				continue
			}

			log.Log.Error(err, "failed to mark pod status", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "status", pod.Status.Phase)
			return fmt.Errorf("failed to mark pod status: %w", err)
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		log.Log.Error(lastErr, "failed to mark pod status after retries", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "status", pod.Status.Phase)
		return fmt.Errorf("failed to mark pod status after retries: %w", lastErr)
	}

	webhook, err := w.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, errMsgWebhookDetails, "runId", dagRunId)
	} else if webhook.URL != "" {
		// notify in background
		go w.webhookNotifier.NotifyPodEvent(
			pod.Spec.Containers[0].Name,
			string(pod.Status.Phase),
			dagRunId,
			taskRunId,
			webhook.URL,
			webhook.VerifySSL,
			int(duration),
		)
	}

	log.Log.Info("pod status written to db", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "status", pod.Status.Phase)
	return nil
}

func (w *worker) handleUnretryablePod(ctx context.Context, pod *v1.Pod, taskRunId, dagRunId int, exitCode int32) {
	log.Log.Info("pod has reached it max backoffLimit or exit code not recoverable", "podUID", pod.UID, "name", pod.Name, "exitCode", exitCode)

	// Get DAG and task names for metrics
	dagName, taskName, namespace := w.getTaskRunMetricsInfo(ctx, taskRunId)

	if err := w.dbManager.MarkTaskAsFailed(ctx, taskRunId); err != nil {
		log.Log.Error(err, "failed to mark task as failed", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
	}

	// Record metrics for unretryable failure
	metrics.RecordTaskOutcome(namespace, dagName, taskName, "unretryable")

	webhook, err := w.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, errMsgWebhookDetails, "runId", dagRunId)
	} else if webhook.URL != "" {
		go w.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "failed", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	taskNames, err := w.dbManager.MarkConnectingTasksAsSuspended(ctx, dagRunId, taskRunId)
	if err == nil {
		if webhook != nil && webhook.URL != "" {
			for _, taskName := range taskNames {
				log.Log.Info("task marked as suspended", "taskName", taskName)
				go w.webhookNotifier.NotifyTaskRun(taskName, "suspended", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
			}
		}
	} else {
		log.Log.Error(err, "failed to mark connecting tasks as suspended", "taskRunId", taskRunId)
		return
	}

	complete, err := w.checkIfDagRunIsComplete(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to check if dag run is complete", "runId", dagRunId)
		return
	}

	if !complete {
		return
	}

	if err := w.deletePVC(ctx, pod); err != nil {
		log.Log.Error(err, "failed to delete PVC", "pod", pod.Name, "namespace", pod.Namespace, "dagRunId", dagRunId, "status", pod.Status.Phase)
	}
}

func (t *worker) deletePod(ctx context.Context, pod *v1.Pod, removeFinaliser bool) error {
	if removeFinaliser {
		if err := object.RemoveFinalizer(t.clientSet, pod.Name, pod.Namespace, "kontroler/logcollection"); err != nil {
			log.Log.Error(err, "error removing finalizer", "pod", pod.Name, "namespace", pod.Namespace)
		}
	}

	backgroundDeletion := metav1.DeletePropagationBackground
	return t.clientSet.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundDeletion,
	})
}

func (t *worker) handleStartedTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task started", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, errMsgDagRunID, "pod", pod.Name)
		return
	}

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, errMsgWebhookDetails, "runId", dagRunId)
	} else if webhook.URL != "" {
		t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "started", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}
}

func (t *worker) handlePendingTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) bool {
	// Get DAG and task names for metrics
	dagName, taskName, namespace := t.getTaskRunMetricsInfo(ctx, taskRunId)

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, errMsgDagRunID, "pod", pod.Name)
		return true
	}

	if hasConfigError(pod) {
		log.Log.Info("detected config error, treating as failure", "podUID", pod.UID, "name", pod.Name)
		// Record metrics for config error state transition
		metrics.RecordTaskOutcome(namespace, dagName, taskName, "config_error")

		// Treat config error as a special kind of failure
		t.handleConfigError(ctx, pod, taskRunId, dagRunId)
		return false
	}

	log.Log.Info("task pending", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, errMsgWebhookDetails, "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "pending", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	return true
}

func (t *worker) handleConfigError(ctx context.Context, pod *v1.Pod, taskRunId, dagRunId int) {
	// Config errors are typically unrecoverable, so mark as failed immediately
	if err := t.dbManager.MarkTaskAsFailed(ctx, taskRunId); err != nil {
		log.Log.Error(err, "failed to mark config error task as failed", "podUID", pod.UID, "name", pod.Name)
	}

	// Mark pod as failed
	if err := t.dbManager.MarkPodStatus(ctx, pod.UID, pod.Name, taskRunId, v1.PodFailed, time.Now(), nil, pod.Namespace); err != nil {
		log.Log.Error(err, "failed to mark pod status", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "status", v1.PodFailed)
	}

	// remove finalizer as no logs will be collected
	if err := t.deletePod(ctx, pod, true); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			log.Log.Error(err, "failed to delete pod with config error", "podUId", pod.UID)
		}
	}

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, errMsgWebhookDetails, "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "failed", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	// Handle downstream tasks
	t.handleUnretryablePod(ctx, pod, taskRunId, dagRunId, -2) // -2 for config error
}

func (t *worker) deletePVC(ctx context.Context, pod *v1.Pod) error {
	for _, volumes := range pod.Spec.Volumes {
		if volumes.PersistentVolumeClaim != nil && volumes.Name == "workspace" {
			// Fetch the PVC
			pvc, err := t.clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, volumes.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			// Remove finalizers
			pvc.Finalizers = []string{}

			// Update the PVC
			_, err = t.clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			return t.clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Delete(ctx, volumes.PersistentVolumeClaim.ClaimName, metav1.DeleteOptions{})
		}
	}

	return nil
}

func (t *worker) checkIfDagRunIsComplete(ctx context.Context, runId int) (bool, error) {
	// check if all tasks are done
	allTasksDone, err := t.dbManager.CheckIfAllTasksDone(ctx, runId)
	if err != nil {
		log.Log.Error(err, "failed to check if all tasks are done", "runId", runId)
		return false, err
	}

	return allTasksDone, nil
}

func (t *worker) getTaskRunID(pod *v1.Pod) (int, error) {
	taskRunIdStr, ok := pod.Annotations[kontrolerTaskRunID]
	if !ok {
		return 0, fmt.Errorf(errFormatWithString, ErrMissingAnnotation, kontrolerTaskRunID)
	}

	taskRunId, err := strconv.Atoi(taskRunIdStr)
	if err != nil {
		return 0, fmt.Errorf(errFormatWithString, ErrInvalidTaskRunID, taskRunIdStr)
	}

	return taskRunId, nil
}

func (t *worker) getDagRunID(pod *v1.Pod) (int, error) {
	dagRunStr, ok := pod.Annotations[kontrolerDagRunID]
	if !ok {
		return 0, fmt.Errorf(errFormatWithString, ErrMissingAnnotation, kontrolerDagRunID)
	}

	return strconv.Atoi(dagRunStr)
}

// Helper function to get DAG and task names for metrics
func (w *worker) getTaskRunMetricsInfo(ctx context.Context, taskRunId int) (dagName, taskName, namespace string) {
	// Get DAG name, task name, and namespace for metrics
	dagName, taskName, namespace, err := w.dbManager.GetTaskRunInfo(ctx, taskRunId)
	if err != nil {
		log.Log.Error(err, "failed to get task run info for metrics", "taskRunId", taskRunId)
		// Use fallback values if DB query fails
		return "unknown", "unknown", "unknown"
	}
	return dagName, taskName, namespace
}

func (w *worker) handleLogCollection(ctx context.Context, pod *v1.Pod) {
	if w.logStore == nil {
		return
	}

	if !w.isPodReadyForLogCollection(pod) {
		return
	}

	// Attempt to get logs, but we don't stop if we can't get them
	go w.performLogCollection(ctx, pod)
}

func (w *worker) isPodReadyForLogCollection(pod *v1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
			return true
		}
	}
	return false
}

func (w *worker) performLogCollection(ctx context.Context, pod *v1.Pod) {
	dagRunId, err := w.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, errMsgDagRunID, "pod", pod.Name)
		return
	}

	if w.isAlreadyFetching(dagRunId, pod) {
		return
	}

	if err := w.logStore.MarkAsFetching(dagRunId, pod); err != nil {
		log.Log.Info("already fetching", "podUID", pod.UID, "name", pod.Name)
		return
	}

	defer w.logStore.UnlistFetching(dagRunId, pod)

	w.uploadLogsAndCleanup(ctx, dagRunId, pod)
}

func (w *worker) isAlreadyFetching(dagRunId int, pod *v1.Pod) bool {
	if ok := w.logStore.IsFetching(dagRunId, pod); ok {
		log.Log.Info("already fetching", "podUID", pod.UID, "name", pod.Name)
		return true
	}
	return false
}

func (w *worker) uploadLogsAndCleanup(ctx context.Context, dagRunId int, pod *v1.Pod) {
	log.Log.Info("started collecting logs", "pod", pod.Name)
	if err := w.logStore.UploadLogs(ctx, dagRunId, w.clientSet, pod); err != nil {
		log.Log.Error(err, "failed to uploadLogs")
	}

	// Check if dagrun still exists after log collection
	exists, err := w.dbManager.DagrunExists(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to check if dagrun exists", "dagrunId", dagRunId)
		return
	}

	if !exists {
		w.cleanupLogsForDeletedDagrun(ctx, dagRunId)
	}
}

func (w *worker) cleanupLogsForDeletedDagrun(ctx context.Context, dagRunId int) {
	if err := w.logStore.DeleteLogs(ctx, dagRunId); err != nil {
		log.Log.Error(err, "failed to delete logs for deleted dagrun", "dagrunId", dagRunId)
	}
	log.Log.Info("deleted logs for non-existent dagrun", "dagrunId", dagRunId)
}

func hasConfigError(pod *v1.Pod) bool {
	// Check regular containers
	for _, status := range pod.Status.ContainerStatuses {
		if isContainerError(status.State) {
			return true
		}
	}

	// Check init containers
	for _, status := range pod.Status.InitContainerStatuses {
		if isContainerError(status.State) {
			return true
		}
	}

	return false
}

func isContainerError(state v1.ContainerState) bool {
	if state.Waiting == nil {
		return false
	}

	switch state.Waiting.Reason {
	case container.StateContainerCreating, container.StatePodInitializing:
		return false
	case container.StateCreateContainerError,
		container.StateRunContainerError,
		container.StateConfigError,
		container.StateErrImagePull,
		container.StateImagePullBackOff:
		return true
	default:
		return false
	}
}

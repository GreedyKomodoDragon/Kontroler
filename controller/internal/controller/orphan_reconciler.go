package controller

import (
	"context"
	"fmt"
	"time"

	"kontroler-controller/internal/db"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// RunOrphanReconciler periodically scans pods managed by Kontroler and deletes pods that are not owned by active task runs in DB.
// It uses an age threshold to avoid deleting freshly-created pods that may be in the process of being claimed.
func RunOrphanReconciler(ctx context.Context, clientset kubernetes.Interface, dbManager db.DBDAGManager, interval time.Duration) error {
	logger := log.FromContext(ctx).WithName("orphan-reconciler")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// only delete pods older than this duration
	const orphanPodGrace = 5 * time.Minute

	selector := "managed-by=kontroler,kontroler/type=task"

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping orphan reconciler")
			return nil
		case <-ticker.C:
			pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				logger.Error(err, "failed to list pods for orphan reconciler")
				continue
			}

			now := time.Now()
			for _, pod := range pods.Items {
				// skip pods younger than grace period
				if pod.CreationTimestamp.Time.Add(orphanPodGrace).After(now) {
					continue
				}

				v, ok := pod.Annotations["kontroler/task-rid"]
				if !ok || v == "" {
					// nothing we can do
					continue
				}

				var taskRunId int
				_, err := fmt.Sscan(v, &taskRunId)
				if err != nil {
					logger.Error(err, "invalid taskRunId annotation", "pod", pod.Name, "ns", pod.Namespace)
					continue
				}

				status, err := dbManager.GetTaskRunStatus(ctx, taskRunId)
				if err != nil {
					// if DB says not found, treat as orphan
					logger.Info("task run not found, deleting pod", "pod", pod.Name, "ns", pod.Namespace, "taskRunId", taskRunId)
					// remove finalizers then delete
					p := pod.DeepCopy()
					p.Finalizers = []string{}
					if _, err := clientset.CoreV1().Pods(pod.Namespace).Update(ctx, p, metav1.UpdateOptions{}); err != nil {
						logger.Error(err, "failed to remove finalizers from orphan pod", "pod", pod.Name)
					}
					_ = clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
					continue
				}

				// if taskRun not in running/pending, delete pod
				if status != "running" && status != "pending" {
					logger.Info("deleting pod for non-active task run", "pod", pod.Name, "ns", pod.Namespace, "taskRunId", taskRunId, "status", status)
					p := pod.DeepCopy()
					p.Finalizers = []string{}
					if _, err := clientset.CoreV1().Pods(pod.Namespace).Update(ctx, p, metav1.UpdateOptions{}); err != nil {
						logger.Error(err, "failed to remove finalizers from pod before delete", "pod", pod.Name)
					}
					_ = clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
				}
			}
		}
	}
}

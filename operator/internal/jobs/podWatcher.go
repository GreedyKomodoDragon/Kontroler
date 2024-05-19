package jobs

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type JobWatcher interface {
	StartWatcher(namespace string) error
	IsWatching(namespace string) bool
}

type jobWatcher struct {
	clientSet  *kubernetes.Clientset
	watcherMap map[string]bool
}

func NewJobWatcher(clientSet *kubernetes.Clientset) JobWatcher {
	return &jobWatcher{
		clientSet:  clientSet,
		watcherMap: map[string]bool{},
	}
}

func (p *jobWatcher) StartWatcher(namespace string) error {
	labelSelector := labels.Set(map[string]string{
		"managed-by": "kubeconductor",
	}).AsSelector()

	// Set up job watcher
	watcher, err := p.clientSet.BatchV1().Jobs(namespace).Watch(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		log.Log.Error(err, "failed to watch jobs", "namespace", namespace)
		return err
	}

	p.watcherMap[namespace] = true

	go func() {
		defer watcher.Stop()
		defer delete(p.watcherMap, namespace)

		// Handle job events
		for event := range watcher.ResultChan() {
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				log.Log.Error(err, "failed to parse job object")
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				if job.Status.Succeeded > 0 {
					fmt.Println("the job succeeded!!")
					continue
				}

				if job.Status.Failed > 0 {
					fmt.Println("the job failed!!")
					// Extract failure details
					pods, err := p.clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: fmt.Sprintf("job-name=%s", job.Name),
					})
					if err != nil {
						log.Log.Error(err, "failed to list pods for job", "job", job.Name)
						continue
					}

					for _, pod := range pods.Items {
						for _, containerStatus := range pod.Status.ContainerStatuses {
							if containerStatus.State.Terminated != nil {
								fmt.Printf("Container %s exit code: %d\n", containerStatus.Name, containerStatus.State.Terminated.ExitCode)
							}
						}
					}
				}
			case watch.Deleted:
				// Job was deleted
				fmt.Printf("Job %s was deleted\n", job.Name)
			}
		}
	}()

	return nil
}

func (p *jobWatcher) IsWatching(namespace string) bool {
	_, ok := p.watcherMap[namespace]
	return ok
}

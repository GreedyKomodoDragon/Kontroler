package jobs

import (
	"context"
	"fmt"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type JobWatcher interface {
	StartWatching()
}

type jobWatcher struct {
	jobAllocator JobAllocator
	dbManager    db.DbManager
	kubeWatcher  watch.Interface
	clientSet    *kubernetes.Clientset
	namespace    string
}

func NewJobWatcher(namespace string, clientSet *kubernetes.Clientset, kubeWatcher watch.Interface, jobAllocator JobAllocator, dbManager db.DbManager) JobWatcher {
	return &jobWatcher{
		jobAllocator: jobAllocator,
		dbManager:    dbManager,
		kubeWatcher:  kubeWatcher,
		namespace:    namespace,
		clientSet:    clientSet,
	}
}

func (j *jobWatcher) StartWatching() {
	// Handle job events
	for event := range j.kubeWatcher.ResultChan() {
		job, ok := event.Object.(*batchv1.Job)
		if !ok {
			log.Log.Error(fmt.Errorf("invalid object"), "failed to parse job object")
			continue
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			log.Log.Info("job event", "jobUid", job.UID, "event.type", event.Type)

			if job.Status.Succeeded > 0 {
				log.Log.Info("jobs successfully finished", "jobUid", job.UID)

				jobId, ok := job.Annotations["kubeconductor/schedule-uid"]
				if !ok {
					log.Log.Error(fmt.Errorf("missing annotation"), "found successful pod missing kubeconductor/schedule-uid", "job", job.Name)
					continue
				}

				jobUid := types.UID(jobId)
				if err := j.dbManager.MarkRunOutcome(context.TODO(), jobUid, "successful"); err != nil {
					log.Log.Error(err, "found pod missing kubeconductor/schedule-uid", "job", job.Name)
				}

				continue
			}

			if job.Status.Failed > 0 {
				log.Log.Info("jobs failed to finish", "jobUid", job.UID)

				// Extract failure details
				pods, err := j.clientSet.CoreV1().Pods(j.namespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: fmt.Sprintf("job-name=%s", job.Name),
				})
				if err != nil {
					// TODO: Mark in DB as missing pods
					log.Log.Error(err, "failed to list pods for job", "job", job.Name)
					continue
				}

				log.Log.Info("number of pods in job", "jobUid", job.UID, "count", len(pods.Items))

				// Get Exitable codes to restart with and backoff limit
				for _, pod := range pods.Items {
					jobId, ok := job.Annotations["kubeconductor/schedule-uid"]
					if !ok {
						// TODO: Mark in DB as missing annotations
						log.Log.Error(err, "found pod missing kubeconductor/schedule-uid", "job", job.Name)
						continue
					}

					if len(pod.Status.ContainerStatuses) == 0 {
						// TODO: Mark in DB as no containers
						log.Log.Error(err, "does not seem to be any containers?", "job", job.Name)
						continue
					}

					if pod.Status.ContainerStatuses[0].State.Terminated == nil {
						// TODO: Mark in DB as missing status information on pod
						log.Log.Error(err, "missing status informationt", "job", job.Name)
						continue
					}

					jobUid := types.UID(jobId)
					ok, err := j.dbManager.ShouldRerun(context.Background(), jobUid, pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
					if err != nil {
						// TODO: Mark in DB as failed
						log.Log.Error(err, "failed to determine if pod should be re-ran", "job", job.Name)
						continue
					}

					if !ok {
						log.Log.Info("job has reached it max backoffLimit or exit code not recoverable", "jobUid", jobUid, "exitCode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)

						if err := j.dbManager.MarkRunOutcome(context.TODO(), jobUid, "failed"); err != nil {
							log.Log.Error(err, "found pod missing kubeconductor/schedule-uid", "job", job.Name)
						}
						continue
					}

					if err := j.dbManager.IncrementRunCount(context.Background(), jobUid); err != nil {
						log.Log.Error(err, "failed to increment failure run count", "job", job.Name)
						continue
					}

					container := pod.Spec.Containers[0]
					_, podName, err := j.jobAllocator.AllocateJob(context.Background(), jobUid, container.Name, container.Image, container.Command, container.Args, pod.Namespace)
					if err != nil {
						// TODO: Mark this as the job failing!
						log.Log.Error(err, "failed to allocate new pod")
						continue
					}

					if err := j.dbManager.AddPodToRun(context.TODO(), podName, jobUid); err != nil {
						log.Log.Error(err, "failed to add pod to run")
						continue
					}

					log.Log.Info("new job allocated", "jobUid", jobUid)
				}
			}
		case watch.Deleted:
			// Job was deleted
			fmt.Printf("Job %s was deleted\n", job.Name)
		}
	}
}

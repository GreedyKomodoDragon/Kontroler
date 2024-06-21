package scheduler

import (
	"context"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/jobs"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/utils"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type SchedulerManager interface {
	Run()
}

type schedulerManager struct {
	jobAllocator   jobs.JobAllocator
	jobWatcher     jobs.JobWatcherFactory
	dbManager      db.DBSchedulerManager
	labelSelectors labels.Selector
}

func NewScheduleManager(jobAllocator jobs.JobAllocator, jobWatcher jobs.JobWatcherFactory, dbManager db.DBSchedulerManager) SchedulerManager {
	return &schedulerManager{
		jobAllocator: jobAllocator,
		dbManager:    dbManager,
		jobWatcher:   jobWatcher,
		labelSelectors: labels.Set(map[string]string{
			"managed-by":         "kubeconductor",
			"kubeconductor/type": "cronjob",
		}).AsSelector(),
	}
}

func (s *schedulerManager) Run() {
	tmr := time.NewTimer(time.Minute)
	for {
		<-tmr.C
		log.Log.Info("timer up, begun looking for cronjobs")

		jobs, err := s.dbManager.GetCronJobsToStart(context.Background())
		if err != nil {
			tmr.Reset(time.Minute)
			continue
		}

		log.Log.Info("number of jobs found", "count", len(jobs))

		for _, job := range jobs {
			// Start watcher first
			if ok := s.jobWatcher.IsWatching(job.Namespace); !ok {
				if err := s.jobWatcher.StartWatcher(job.Namespace, s.labelSelectors); err != nil {
					log.Log.Error(err, "failed to start watching namespace for pods", "namespace", job.Namespace)
				}

				log.Log.Info("started watching new namespace", "namespace", job.Namespace)
			}

			name := "-" + utils.GenerateRandomName()
			newUUID, err := uuid.NewUUID()
			if err != nil {
				log.Log.Error(err, "failed to generate uuid")
				continue
			}

			runID := types.UID(newUUID.String())

			id, err := s.jobAllocator.AllocateJob(context.Background(), runID, string(job.Id)+name, job.ImageName, job.Command, job.Args, job.Namespace)
			if err != nil {
				// TODO: Mark this as the job failing!
				log.Log.Error(err, "failed to allocate a new pod", "runId", runID, "jobNamespace", job.Namespace, "jobId", job.Id)
				continue
			}

			log.Log.Info("new pod allocated", "namespace", job.Namespace)

			if err := s.dbManager.UpdateNextTime(context.Background(), job.Id, job.Schedule); err != nil {
				log.Log.Error(err, "failed to update next time", "podId", id)
			}

			if err := s.dbManager.StartRun(context.Background(), job.Id, runID); err != nil {
				log.Log.Error(err, "failed to mark job as started", "runID", runID, "jobId", job.Id)
			}

		}

		tmr.Reset(time.Minute)
	}
}

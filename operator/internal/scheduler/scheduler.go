package scheduler

import (
	"context"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/jobs"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type SchedulerManager interface {
	Run()
}

type schedulerManager struct {
	jobAllocator jobs.JobAllocator
	jobWatcher   jobs.JobWatcher
	dbManager    db.DbManager
}

func NewScheduleManager(jobAllocator jobs.JobAllocator, jobWatcher jobs.JobWatcher, dbManager db.DbManager) SchedulerManager {
	return &schedulerManager{
		jobAllocator: jobAllocator,
		dbManager:    dbManager,
		jobWatcher:   jobWatcher,
	}
}

func (s *schedulerManager) Run() {
	tmr := time.NewTimer(time.Minute)
	for {
		<-tmr.C

		jobs, err := s.dbManager.GetCronJobsToStart(context.Background())
		if err != nil {
			tmr.Reset(time.Minute)
			continue
		}

		for _, job := range jobs {
			// Start watcher first
			if ok := s.jobWatcher.IsWatching("operator-system"); !ok {
				if err := s.jobWatcher.StartWatcher("operator-system"); err != nil {
					log.Log.Error(err, "failed to start watching namespace for pods", "namespace", "operator-system")
				}

				log.Log.Info("started watching new namespace", "namespace", "operator-system")
			}

			name := "-" + generateRandomName()
			newUUID, err := uuid.NewUUID()
			if err != nil {
				log.Log.Error(err, "failed to generate uuid")
			}

			runID := types.UID(newUUID.String())

			id, err := s.jobAllocator.AllocateJob(context.Background(), runID, string(job.Id)+name, job.ImageName, job.Command, job.Args, "operator-system")
			if err != nil {
				log.Log.Error(err, "failed to allocate a new pod")
				continue
			}

			log.Log.Info("new pod allocated", "namespace", "operator-system")

			if err := s.dbManager.UpdateNextTime(context.Background(), job.Id, job.Schedule); err != nil {
				log.Log.Error(err, "failed to update next time", "podId", id)
			}

			if err := s.dbManager.StartRun(context.Background(), job.Id, runID); err != nil {
				log.Log.Error(err, "failed to mark job as started", "runID", runID)
			}
		}

		tmr.Reset(time.Minute)
	}

}

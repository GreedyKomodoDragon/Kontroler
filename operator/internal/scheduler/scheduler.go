package scheduler

import (
	"context"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/pods"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type SchedulerManager interface {
	Run()
}

type schedulerManager struct {
	podAllocator pods.PodAllocator
	dbManager    db.DbManager
}

func NewScheduleManager(podAllocator pods.PodAllocator, dbManager db.DbManager) SchedulerManager {
	return &schedulerManager{
		podAllocator: podAllocator,
		dbManager:    dbManager,
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
			name := "-" + generateRandomName()
			id, err := s.podAllocator.AllocatePod(context.Background(), job.Id, string(job.Id)+name, job.ImageName, job.Command, "operator-system")
			if err != nil {
				log.Log.Error(err, "failed to allocate a new pod")
				continue
			}

			if err := s.dbManager.UpdateNextTime(context.Background(), job.Id, job.Schedule); err != nil {
				log.Log.Error(err, "failed to update next time", "podId", id)
			}

		}

		tmr.Reset(time.Minute)
	}

}

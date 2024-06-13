package dag

import (
	"context"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// DagScheduler will every min run a check on the Database to determine if a dag should be started
// For example, this could be based on a CronJob Schedule or a time window
type DagScheduler interface {
	Run()
}

type dagscheduler struct {
	dbManager     db.DBDAGManager
	taskAllocator TaskAllocator
}

func NewDagScheduler(dbManager db.DBDAGManager, taskAllocator TaskAllocator) DagScheduler {
	return &dagscheduler{
		dbManager:     dbManager,
		taskAllocator: taskAllocator,
	}
}

func (d *dagscheduler) Run() {
	tmr := time.NewTimer(time.Minute)
	for {
		<-tmr.C
		log.Log.Info("timer up, begun looking for dags to run")

		ctx := context.Background()
		dagIds, err := d.dbManager.GetDAGsToStartAndUpdate(ctx)
		if err != nil {
			tmr.Reset(time.Minute)
			continue
		}

		log.Log.Info("number of dags found", "count", len(dagIds))

		for _, dagId := range dagIds {
			tasks, err := d.dbManager.GetStartingTasks(ctx, dagId)
			if err != nil {
				log.Log.Error(err, "failed to get starting tasks for dag", "dag_id", dagId)
				continue
			}

			// Provide task to allocator
			for _, task := range tasks {
				if err := d.taskAllocator.AllocateTask(task); err != nil {
					log.Log.Error(err, "failed to allocate task to job", "dag_id", dagId, "task_id", task.Id)
					continue
				}
			}

		}

		tmr.Reset(time.Minute)
	}

}

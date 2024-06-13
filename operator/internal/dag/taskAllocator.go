package dag

import "github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"

type TaskAllocator interface {
	AllocateTask(db.Task) error
}

type taskAllocator struct {
}

func NewTaskAllocator() TaskAllocator {
	return &taskAllocator{}
}

func (*taskAllocator) AllocateTask(db.Task) error {
	return nil
}

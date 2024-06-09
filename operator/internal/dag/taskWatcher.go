package dag

// Purpose of TaskWatcher is to listen for pods within a job to finish and record results/trigger the next pods
type TaskWatcher interface {
	StartWatching()
}

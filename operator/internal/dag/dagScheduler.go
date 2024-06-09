package dag

// DagScheduler will every min run a check on the Database to determine if a dag should be started
// For example, this could be based on a CronJob Schedule or a time window
type DagScheduler interface {
	Run()
}

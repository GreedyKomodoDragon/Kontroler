package queue

type Queue interface {
	Push(value string) error
	PushBatch(values []string) error
	Pop() (string, error)
	PopBatch(count int) ([]string, error)
	Size() (uint64, error)
	Close() error
}

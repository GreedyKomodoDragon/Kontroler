package container

const (
	// Container states that indicate normal startup
	StateContainerCreating = "ContainerCreating"
	StatePodInitializing   = "PodInitializing"

	// Container error states
	StateCreateContainerError = "CreateContainerError"
	StateRunContainerError    = "RunContainerError"
	StateConfigError          = "CreateContainerConfigError"
	StateErrImagePull         = "ErrImagePull"
	StateImagePullBackOff     = "ImagePullBackOff"
)

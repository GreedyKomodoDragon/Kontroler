package dag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type TaskAllocator interface {
	AllocateTask(context.Context, db.Task, int, int, string) (types.UID, error)
	AllocateTaskWithEnv(context.Context, db.Task, int, int, string, []v1.EnvVar, *v1.ResourceRequirements) (types.UID, error)
	CreateEnvs(task db.Task) *[]v1.EnvVar
}

type taskAllocator struct {
	clientSet *kubernetes.Clientset
	id        string
}

func NewTaskAllocator(clientSet *kubernetes.Clientset, id string) TaskAllocator {
	return &taskAllocator{
		clientSet: clientSet,
		id:        id,
	}
}

func (t *taskAllocator) AllocateTask(ctx context.Context, task db.Task, dagRunId, taskRunId int, namespace string) (types.UID, error) {
	envs := t.CreateEnvs(task)
	return t.allocatePod(ctx, task, dagRunId, taskRunId, namespace, *envs, nil)
}

func (t *taskAllocator) AllocateTaskWithEnv(ctx context.Context, task db.Task, dagRunId, taskRunId int, namespace string, envs []v1.EnvVar, resources *v1.ResourceRequirements) (types.UID, error) {
	return t.allocatePod(ctx, task, dagRunId, taskRunId, namespace, envs, resources)
}

func (t *taskAllocator) allocatePod(ctx context.Context, task db.Task, dagRunId, taskRunId int, namespace string, envs []v1.EnvVar, resources *v1.ResourceRequirements) (types.UID, error) {
	podSpec := v1.PodSpec{
		RestartPolicy: v1.RestartPolicyNever,
		Volumes:       []v1.Volume{},
	}

	if task.Script != "" {
		vol := v1.Volume{
			Name: "shared-scripts",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		}

		// We check as AllocateTaskWithEnv re-uses the mounts to avoid going to the database
		// Downside is that it will create two volumes with the same values
		if task.PodTemplate == nil || (task.PodTemplate != nil && !containsVolume(task.PodTemplate.Volumes, vol)) {
			podSpec.Volumes = append(podSpec.Volumes, vol)
		}

		scriptInjectorImage := task.ScriptInjectorImage
		if scriptInjectorImage == "" {
			// Kontroler has tests and uses UBI images/recommends as base for best experience with kontroler
			scriptInjectorImage = "registry.access.redhat.com/ubi9/ubi-micro:latest"
		}

		// We only support bash, so any container you need to use e.g. scriptInjectorImage or task.Image
		// Needs to have bash installed. ubuntu + UBI8 both *should* work
		podSpec.InitContainers = []v1.Container{
			{
				Name:  "script-copier",
				Image: scriptInjectorImage,
				Command: []string{
					"bash", "-c", fmt.Sprintf(`printf %s > /tmp/my-script.sh && echo "Script created" || echo "Failed to write script" >&2 &&
						chmod 555 /tmp/my-script.sh && echo "Permissions set" || echo "Failed to set permissions" >&2`, shellescape.Quote(task.Script)),
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "shared-scripts",
						MountPath: "/tmp",
					},
				},
			},
		}

		podSpec.Containers = []v1.Container{
			{
				Name:    task.Name,
				Image:   task.Image,
				Command: []string{"bash", "-c", "/tmp/my-script.sh"},
				Env:     envs,
			},
		}

		mount := v1.VolumeMount{
			Name:      "shared-scripts",
			MountPath: "/tmp",
			ReadOnly:  true,
		}

		// We check as AllocateTaskWithEnv re-uses the mounts to avoid going to the database
		// Downside is that it will create two volumes with the same values
		if task.PodTemplate == nil || (task.PodTemplate != nil && !containsVolumeMount(task.PodTemplate.VolumeMounts, mount)) {
			podSpec.Containers[0].VolumeMounts = []v1.VolumeMount{mount}
		}

	} else {
		podSpec.Containers = []v1.Container{
			{
				Name:    task.Name,
				Image:   task.Image,
				Command: task.Command,
				Args:    task.Args,
				Env:     envs,
			},
		}
	}

	// Apply PodTemplate if provided
	if task.PodTemplate != nil {
		t.applyPodTemplate(&podSpec, &task)
	}

	// Override resources if provided
	if resources != nil {
		podSpec.Containers[0].Resources = *resources
	}

	// Create pod metadata and pod object
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"managed-by":     "kontroler",
				"kontroler/type": "task",
				"kontroler/id":   t.id,
			},
			Annotations: map[string]string{
				"kontroler/task-rid":  strconv.Itoa(taskRunId),
				"kontroler/dagRun-id": strconv.Itoa(dagRunId),
				"kontroler/task-id":   strconv.Itoa(task.Id),
			},
			Finalizers: []string{"kontroler/logcollection"},
		},
		Spec: podSpec,
	}

	// Attempt pod creation with retry on name collision
	for i := 0; i < 5; i++ {
		pod.ObjectMeta.Name = utils.GenerateRandomName()

		createdPod, err := t.clientSet.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				continue
			} else {
				return "", err
			}
		}

		return createdPod.UID, nil
	}

	return "", fmt.Errorf("failed to create pod due to naming collisions")
}

// Helper function to apply PodTemplate attributes to the pod spec
func (t *taskAllocator) applyPodTemplate(podSpec *v1.PodSpec, task *db.Task) {
	podSpec.Volumes = append(podSpec.Volumes, task.PodTemplate.Volumes...)
	podSpec.ImagePullSecrets = task.PodTemplate.ImagePullSecrets
	podSpec.SecurityContext = task.PodTemplate.SecurityContext
	podSpec.NodeSelector = task.PodTemplate.NodeSelector
	podSpec.Tolerations = task.PodTemplate.Tolerations
	podSpec.Affinity = task.PodTemplate.Affinity
	podSpec.ServiceAccountName = task.PodTemplate.ServiceAccountName
	podSpec.AutomountServiceAccountToken = task.PodTemplate.AutomountServiceAccountToken
	podSpec.ActiveDeadlineSeconds = task.PodTemplate.ActiveDeadlineSeconds

	if podSpec.Containers[0].VolumeMounts == nil {
		podSpec.Containers[0].VolumeMounts = task.PodTemplate.VolumeMounts
	} else {
		podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, task.PodTemplate.VolumeMounts...)
	}

	if task.PodTemplate.Resources != nil {
		podSpec.Containers[0].Resources = *task.PodTemplate.Resources
	}
}

func (t *taskAllocator) CreateEnvs(task db.Task) *[]v1.EnvVar {
	envs := []v1.EnvVar{}
	for _, param := range task.Parameters {
		if param.IsSecret {
			envs = append(envs, v1.EnvVar{
				Name: param.Name,
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: param.Value,
						},
						// Current Version will also look into Key "secret"
						Key: "secret",
					},
				},
			})
		} else {
			envs = append(envs, v1.EnvVar{
				Name:  param.Name,
				Value: param.Value,
			})
		}
	}

	return &envs
}

func containsVolumeMount(slice []v1.VolumeMount, item v1.VolumeMount) bool {
	for _, v := range slice {
		if v.Name == item.Name && v.MountPath == item.MountPath && v.ReadOnly == item.ReadOnly {
			return true
		}
	}
	return false
}

func containsVolume(slice []v1.Volume, item v1.Volume) bool {
	for _, v := range slice {
		if v.Name == item.Name {
			return true
		}
	}
	return false
}

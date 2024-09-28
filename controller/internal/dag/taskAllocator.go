package dag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"

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
}

func NewTaskAllocator(clientSet *kubernetes.Clientset) TaskAllocator {
	return &taskAllocator{
		clientSet: clientSet,
	}
}

func (t *taskAllocator) AllocateTask(ctx context.Context, task db.Task, dagRunId, taskRunId int, namespace string) (types.UID, error) {
	envs := t.CreateEnvs(task)

	podSpec := v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:    task.Name,
				Image:   task.Image,
				Command: task.Command,
				Args:    task.Args,
				Env:     *envs,
			},
		},
		RestartPolicy: v1.RestartPolicyNever,
	}

	if task.PodTemplate != nil {
		podSpec.Volumes = task.PodTemplate.Volumes
		podSpec.ImagePullSecrets = task.PodTemplate.ImagePullSecrets
		podSpec.SecurityContext = task.PodTemplate.SecurityContext
		podSpec.NodeSelector = task.PodTemplate.NodeSelector
		podSpec.Tolerations = task.PodTemplate.Tolerations
		podSpec.Affinity = task.PodTemplate.Affinity
		podSpec.ServiceAccountName = task.PodTemplate.ServiceAccountName
		podSpec.AutomountServiceAccountToken = task.PodTemplate.AutomountServiceAccountToken
		podSpec.Containers[0].VolumeMounts = task.PodTemplate.VolumeMounts

		if task.PodTemplate.Resources != nil {
			podSpec.Containers[0].Resources = *task.PodTemplate.Resources
		}
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"managed-by":     "kontroler",
				"kontroler/type": "task",
			},
			Annotations: map[string]string{
				"kontroler/task-rid":  strconv.Itoa(taskRunId),
				"kontroler/dagRun-id": strconv.Itoa(dagRunId),
			},
		},
		Spec: podSpec,
	}

	// TODO: make this dynamic
	for i := 0; i < 5; i++ {
		pod.ObjectMeta.Name = utils.GenerateRandomName()

		createdPod, err := t.clientSet.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				// Name collision, retry with a new name
				continue
			} else {
				// For any other error, return immediately to avoid multiple pod creation
				return "", err
			}
		}

		// If the pod is created successfully, return its UID
		return createdPod.UID, nil
	}

	return "", fmt.Errorf("failed to create pod due to naming collisions")
}

func (t *taskAllocator) AllocateTaskWithEnv(ctx context.Context, task db.Task, dagRunId, taskRunId int, namespace string, envs []v1.EnvVar, resources *v1.ResourceRequirements) (types.UID, error) {
	containerSpec := []v1.Container{
		{
			Name:    task.Name,
			Image:   task.Image,
			Command: task.Command,
			Args:    task.Args,
			Env:     envs,
		},
	}

	if resources != nil {
		containerSpec[0].Resources = *resources
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"managed-by":     "kontroler",
				"kontroler/type": "task",
			},
			Annotations: map[string]string{
				"kontroler/task-rid":  strconv.Itoa(taskRunId),
				"kontroler/dagRun-id": strconv.Itoa(dagRunId),
			},
		},
		Spec: v1.PodSpec{
			Containers:    containerSpec,
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	// TODO: make this dynamic
	for i := 0; i < 5; i++ {
		pod.ObjectMeta.Name = utils.GenerateRandomName()

		createdJob, err := t.clientSet.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				// Name collision, retry with a new name
				continue
			} else {
				// For any other error, return immediately to avoid multiple pod creation
				return "", err
			}
		}

		log.Log.Info("created pod", "podUID", createdJob.UID, "name", createdJob.Name)

		// If the job is created successfully, return its UID
		return createdJob.UID, nil
	}

	return "", fmt.Errorf("failed to create pod due to naming collisions")
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

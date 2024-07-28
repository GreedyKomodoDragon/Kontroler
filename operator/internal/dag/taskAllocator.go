package dag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type TaskAllocator interface {
	AllocateTask(context.Context, db.Task, int, int, string) (types.UID, error)
	AllocateTaskWithEnv(context.Context, db.Task, int, int, string, []v1.EnvVar) (types.UID, error)
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
	backoff := int32(0)

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

	job := &batchv1.Job{
		// TODO: Refactor this to enable it to be re-used in DAG task
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"managed-by":         "kubeconductor",
				"kubeconductor/type": "task",
			},
			Annotations: map[string]string{
				"kubeconductor/task-rid":  strconv.Itoa(taskRunId),
				"kubeconductor/dagRun-id": strconv.Itoa(dagRunId),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    task.Name,
							Image:   task.Image,
							Command: task.Command,
							Args:    task.Args,
							Env:     envs,
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	// TODO: make this dynamic
	for i := 0; i < 5; i++ {
		job.ObjectMeta.Name = utils.GenerateRandomName()

		// Create the Job
		// TODO: Make namespace more dynamic
		createdJob, err := t.clientSet.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				// just try again with a new name
				continue
			}

			return "", err
		}

		return createdJob.UID, nil
	}

	return "", fmt.Errorf("failed to create pod due to naming collisions")
}

func (t *taskAllocator) AllocateTaskWithEnv(ctx context.Context, task db.Task, dagRunId, taskRunId int, namespace string, envs []v1.EnvVar) (types.UID, error) {
	backoff := int32(0)

	job := &batchv1.Job{
		// TODO: Refactor this to enable it to be re-used in DAG task
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"managed-by":         "kubeconductor",
				"kubeconductor/type": "task",
			},
			Annotations: map[string]string{
				"kubeconductor/task-rid":  strconv.Itoa(taskRunId),
				"kubeconductor/dagRun-id": strconv.Itoa(dagRunId),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    task.Name,
							Image:   task.Image,
							Command: task.Command,
							Args:    task.Args,
							Env:     envs,
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	// TODO: make this dynamic
	for i := 0; i < 5; i++ {
		job.ObjectMeta.Name = utils.GenerateRandomName()

		// Create the Job
		// TODO: Make namespace more dynamic
		createdJob, err := t.clientSet.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				// just try again with a new name
				continue
			}

			return "", err
		}

		return createdJob.UID, nil
	}

	return "", fmt.Errorf("failed to create pod due to naming collisions")
}

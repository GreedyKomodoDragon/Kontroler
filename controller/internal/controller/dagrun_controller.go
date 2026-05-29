/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kontroler-controller/api/v1alpha1"
	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/object"
	"kontroler-controller/internal/workers"
)

const (
	dagRunFinalizer = "kontroler.greedykomodo/dagrun.finalizer"
	pvcNameFormat   = "%s-pvc"
)

// DagRunReconciler reconciles a DagRun object
type DagRunReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	DbManager     db.DBDAGManager
	TaskAllocator workers.TaskAllocator
	LogStore      object.LogStore
}

//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dagruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dagruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dagruns/finalizers,verbs=update

func (r *DagRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("reconcile event", "controller", "DagRun", "req.Name", req.Name, "req.Namespace", req.Namespace, "req.NamespacedName", req.NamespacedName)

	// Fetch the Schedule object that triggered the reconciliation
	var dagRun kontrolerv1alpha1.DagRun
	if err := r.Get(ctx, req.NamespacedName, &dagRun); err != nil {
		// Check if the object was deleted
		if err := client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, err
		}

		log.Log.Info("DagRun deleted", "req.Name", req.Name, "req.Namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	// Check if the DagRun is being deleted
	if !dagRun.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &dagRun)
	}

	// Add finalizer if it doesn't exist
	if !containsString(dagRun.Finalizers, dagRunFinalizer) {
		old := dagRun.DeepCopy()
		dagRun.Finalizers = append(dagRun.Finalizers, dagRunFinalizer)
		if err := r.Patch(ctx, &dagRun, client.MergeFrom(old)); err != nil {
			return ctrl.Result{}, err
		}
		// Requeue so the reconcile loop continues and sees the object with the
		// finalizer in place. Because GenerationChangedPredicate filters out
		// metadata-only updates, requeueing ensures we process the object.
		return ctrl.Result{Requeue: true}, nil
	}

	// check if dag exists
	ok, dagId, err := r.DbManager.DagExists(ctx, dagRun.Spec.DagName)
	if err != nil {
		log.Log.Error(err, "failed to check if dag exists", "dag_id", dagRun.Spec.DagName)
		return ctrl.Result{}, err
	}

	if !ok {
		log.Log.Info("dag does not exist", "dagName", dagRun.Spec.DagName)
		return ctrl.Result{}, nil
	}

	// Check if a DagRun with the same parameters already exists
	alreadyExists, err := r.DbManager.FindExistingDAGRun(ctx, dagRun.Name)
	if err != nil {
		log.Log.Error(err, "failed to check for existing DagRun", "dag_id", dagRun.Spec.DagName)
		return ctrl.Result{}, err
	}

	if alreadyExists {
		// log.Log.Info("DagRun with the same name already exists", "dagRun_id", dagRun.Spec.DagName, "dag_name", dagRun.Name)
		return ctrl.Result{}, nil
	}

	parameters, err := r.DbManager.GetDagParameters(ctx, dagRun.Spec.DagName)
	if err != nil {
		log.Log.Error(err, "failed to find parameters", "dag_id", dagRun.Spec.DagName)
		return ctrl.Result{}, err
	}

	paramMap := map[string]v1alpha1.ParameterSpec{}
	for _, param := range dagRun.Spec.Parameters {
		// Don't add it if it is not a valid parameter
		paramDefault, ok := parameters[param.Name]
		if !ok {
			continue
		}

		if param.FromSecret != "" && paramDefault.IsSecret {
			paramDefault.Value = param.FromSecret
			paramMap[param.Name] = param
			continue
		}

		if param.FromSecret == "" && !paramDefault.IsSecret {
			paramDefault.Value = param.Value
			paramMap[param.Name] = param
			continue
		}

		// If you get here the parameter is invalid due to secret/value mismatch
	}

	// fetch pvc details from db
	pvc, err := r.DbManager.GetWorkspacePVCTemplate(ctx, dagId)
	if err != nil {
		log.Log.Error(err, "failed to get workspace details", "dag_id", dagId, "namespace", dagRun.Namespace)
		return ctrl.Result{}, err
	}

	var pvcName *string
	if pvc != nil {
		name, err := r.createPVC(ctx, &dagRun, pvc)
		if err != nil {
			return ctrl.Result{}, err
		}

		pvcName = &name
	}

	runId, err := r.DbManager.CreateDAGRun(ctx, dagRun.Name, &dagRun.Spec, paramMap, pvcName)
	if err != nil {
		log.Log.Error(err, "failed to create dag run entry", "dag_id", dagRun.Spec.DagName)
		return ctrl.Result{}, err
	}

	tasks, err := r.DbManager.GetStartingTasks(ctx, dagRun.Spec.DagName, runId)
	if err != nil {
		log.Log.Error(err, "failed to get starting tasks for dag", "dag_id", dagRun.Spec.DagName)
		return ctrl.Result{}, err
	}

	log.Log.Info("GetStartingTasks", "dag_id", dagRun.Spec.DagName, "tasks_len", len(tasks))

	// Enqueue starting tasks as pending Task_Runs so workers will claim them
	for _, task := range tasks {
		// Update defaults with values from DagRun
		for i := 0; i < len(task.Parameters); i++ {
			if param, ok := paramMap[task.Parameters[i].Name]; ok {
				if task.Parameters[i].IsSecret {
					task.Parameters[i].Value = param.FromSecret
				} else {
					task.Parameters[i].Value = param.Value
				}
			}
		}

		taskRunId, err := r.DbManager.AddPendingTaskRun(ctx, runId, task.Id)
		if err != nil {
			log.Log.Error(err, "failed to add pending task run", "dag_id", dagRun.Spec.DagName, "task_id", task.Id)
			continue
		}

		log.Log.Info("enqueued pending task", "dag_id", dagRun.Spec.DagName, "task_id", task.Id, "taskRunId", taskRunId)
	}

	old := dagRun.DeepCopy()
	dagRun.Status.DagRunId = runId
	if old.Status.DagRunId != dagRun.Status.DagRunId {
		if err := r.Status().Patch(ctx, &dagRun, client.MergeFrom(old)); err != nil {
			log.Log.Error(err, "failed to update DagRun status with runID", "dag_id", dagRun.Spec.DagName)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DagRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kontrolerv1alpha1.DagRun{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func (r *DagRunReconciler) createPVC(ctx context.Context, dagRun *kontrolerv1alpha1.DagRun, pvcTemplate *v1alpha1.PVC) (string, error) {
	pvcName := fmt.Sprintf(pvcNameFormat, dagRun.Name)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: dagRun.Namespace,
			// added to make sure no one accidentally deletes the PVC
			Finalizers: []string{
				"kontroler.greedykomodo/pvc.finalizer",
			},
		},
		Spec: pvcTemplate.ToK8sPersistentVolumeClaimSpec(),
	}

	// Create the PVC
	if err := r.Client.Create(ctx, pvc); err != nil {
		// ignore if already created
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return "", err
		}

		log.Log.Info("PVC already exists", "pvc", pvc.Name)
		return pvcName, nil
	}

	log.Log.Info("PVC created successfully", "pvc", pvc.Name)
	return pvcName, nil
}

func (r *DagRunReconciler) handleDeletion(ctx context.Context, dagRun *kontrolerv1alpha1.DagRun) (ctrl.Result, error) {
	if !containsString(dagRun.Finalizers, dagRunFinalizer) {
		return ctrl.Result{}, nil
	}

	// suspend the dag run first
	pods, err := r.DbManager.SuspendDagRun(ctx, dagRun.Status.DagRunId)
	if err != nil {
		log.Log.Error(err, "failed to suspend dag run", "dagRunId", dagRun.Status.DagRunId)
		return ctrl.Result{}, err
	}

	// delete pods in parallel with bounded concurrency
	const podConcurrency = defaultConcurrency
	sem := make(chan struct{}, podConcurrency)
	g, gctx := errgroup.WithContext(ctx)
	for _, pod := range pods {
		pod := pod
		sem <- struct{}{}
		g.Go(func() error {
			defer func() { <-sem }()
			if err := deletePodByNameAndNamespace(gctx, r.Client, pod.Name, pod.Namespace); err != nil {
				log.Log.Error(err, "failed to delete pod", "podName", pod.Name, "podNamespace", pod.Namespace)
			}
			return nil
		})
	}
	_ = g.Wait()

	// Delete the DAG run from database
	if err := r.DbManager.DeleteDagRun(ctx, dagRun.Status.DagRunId); err != nil {
		log.Log.Error(err, "failed to delete dag run from database", "dagRunId", dagRun.Status.DagRunId)
		return ctrl.Result{}, err
	}

	// Remove the finalizer
	old := dagRun.DeepCopy()
	dagRun.Finalizers = removeString(dagRun.Finalizers, dagRunFinalizer)
	if err := r.Patch(ctx, dagRun, client.MergeFrom(old)); err != nil {
		return ctrl.Result{}, err
	}

	// remove logs
	if err := r.LogStore.DeleteLogs(ctx, dagRun.Status.DagRunId); err != nil {
		log.Log.Error(err, "failed to delete logs", "dagRunId", dagRun.Status.DagRunId)
		return ctrl.Result{}, err
	}

	// delete the PVC
	pvcName := fmt.Sprintf(pvcNameFormat, dagRun.Name)
	if err := deletePVCByNameAndNamespace(ctx, r.Client, pvcName, dagRun.Namespace); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Log.Error(err, "failed to delete pvc", "pvcName", pvcName, "namespace", dagRun.Namespace)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func deletePVCByNameAndNamespace(ctx context.Context, c client.Client, name string, namespace string) error {
	pvc := &corev1.PersistentVolumeClaim{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	if err := c.Get(ctx, key, pvc); err != nil {
		return err
	}

	// Remove all finalizers from the PVC
	old := pvc.DeepCopy()
	pvc.Finalizers = []string{}
	if err := c.Patch(ctx, pvc, client.MergeFrom(old)); err != nil {
		return err
	}

	// Delete the PVC
	if err := c.Delete(ctx, pvc); err != nil {
		return err
	}

	return nil
}

// Helper functions for finalizer management
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func deletePodByNameAndNamespace(ctx context.Context, c client.Client, name string, namespace string) error {
	// Create a Pod object with only the metadata needed to identify it
	pod := &corev1.Pod{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	if err := c.Get(ctx, key, pod); err != nil {
		return err
	}

	// Remove the actual finaliser (not an annotation)
	var finalisers []string
	for _, f := range pod.ObjectMeta.Finalizers {
		if f != "kontroler/logcollection" {
			finalisers = append(finalisers, f)
		}
	}
	pod.ObjectMeta.Finalizers = finalisers
	old := pod.DeepCopy()
	if err := c.Patch(ctx, pod, client.MergeFrom(old)); err != nil {
		return err
	}

	// Delete the Pod
	if err := c.Delete(ctx, pod); err != nil {
		return err
	}

	return nil
}

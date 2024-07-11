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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeconductorv1alpha1 "github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/dag"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
)

// DagRunReconciler reconciles a DagRun object
type DagRunReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	DbManager     db.DBDAGManager
	TaskAllocator dag.TaskAllocator
}

//+kubebuilder:rbac:groups=kubeconductor.greedykomodo,resources=dagruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubeconductor.greedykomodo,resources=dagruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubeconductor.greedykomodo,resources=dagruns/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DagRun object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *DagRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("reconcile event", "controller", "DagRun", "req.Name", req.Name, "req.Namespace", req.Namespace, "req.NamespacedName", req.NamespacedName)

	// Fetch the Schedule object that triggered the reconciliation
	var dagRun kubeconductorv1alpha1.DagRun
	if err := r.Get(ctx, req.NamespacedName, &dagRun); err != nil {
		// Return error if unable to fetch Schedule object
		return ctrl.Result{}, err
	}

	tasks, err := r.DbManager.GetStartingTasks(ctx, dagRun.Spec.DagId)
	if err != nil {
		log.Log.Error(err, "failed to get starting tasks for dag", "dag_id", dagRun.Spec.DagId)
		return ctrl.Result{}, err
	}

	runId, err := r.DbManager.CreateDAGRun(ctx, dagRun.Spec.DagId)
	if err != nil {
		log.Log.Error(err, "failed to create dag run entry", "dag_id", dagRun.Spec.DagId)
		return ctrl.Result{}, err
	}

	// Provide task to allocator
	for _, task := range tasks {
		taskRunId, err := r.DbManager.MarkTaskAsStarted(ctx, runId, task.Id)
		if err != nil {
			log.Log.Error(err, "failed to mask task as stated", "dag_id", dagRun.Spec.DagId, "task_id", task.Id)
			continue
		}

		if _, err := r.TaskAllocator.AllocateTask(ctx, task, runId, taskRunId); err != nil {
			log.Log.Error(err, "failed to allocate task to job", "dag_id", dagRun.Spec.DagId, "task_id", task.Id)
		}

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DagRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeconductorv1alpha1.DagRun{}).
		Complete(r)
}

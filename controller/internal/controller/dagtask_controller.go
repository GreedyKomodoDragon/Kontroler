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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kontrolerv1alpha1 "github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
)

// DagTaskReconciler reconciles a DagTask object
type DagTaskReconciler struct {
	client.Client
	DbManager db.DBDAGManager
	Scheme    *runtime.Scheme
}

//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dagtasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dagtasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dagtasks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DagTask object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *DagTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("reconcile event", "controller", "dagTask", "req.Name", req.Name, "req.Namespace", req.Namespace, "req.NamespacedName", req.NamespacedName)

	// Fetch the DagTask object that triggered the reconciliation
	var task kontrolerv1alpha1.DagTask
	if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
		// Handle the case where the DAG object was deleted before reconciliation
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		// Return error if unable to fetch DAG object
		return ctrl.Result{}, err
	}

	// Check if the Task is marked for deletion
	if !task.ObjectMeta.DeletionTimestamp.IsZero() {
		// TODO: Check if has finialiser on it, if so ignored/say not allowed
		// TODO: Check that it has no dags using that task:
		//       - if it does => add finialiser to it
		//       - if it does not => (soft?) delete the task
		return ctrl.Result{}, nil
	}

	// Store the DAG object in the database
	if err := r.DbManager.AddTask(ctx, &task, req.NamespacedName.Namespace); err != nil {
		if err.Error() == "applying the same task" {
			log.Log.Info("reconcile event", "controller", "dagTask", "event", "applying the same task")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DagTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kontrolerv1alpha1.DagTask{}).
		Complete(r)
}

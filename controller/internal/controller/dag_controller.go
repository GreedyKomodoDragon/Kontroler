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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kontrolerv1alpha1 "github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
)

// DAGReconciler reconciles a DAG object
type DAGReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	DbManager db.DBDAGManager
}

//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dags,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dags/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=dags/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *DAGReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("reconcile event", "controller", "dag", "req.Name", req.Name, "req.Namespace", req.Namespace, "req.NamespacedName", req.NamespacedName)

	// Fetch the DAG object that triggered the reconciliation
	var dag kontrolerv1alpha1.DAG
	if err := r.Get(ctx, req.NamespacedName, &dag); err != nil {
		// Handle the case where the DAG object was deleted before reconciliation
		if errors.IsNotFound(err) {
			// DAG was deleted, remove it from the database
			if err := r.deleteFromDatabase(ctx, req.NamespacedName); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// Return error if unable to fetch DAG object
		return ctrl.Result{}, err
	}

	// Check if the DAG is marked for deletion
	if !dag.ObjectMeta.DeletionTimestamp.IsZero() {
		// The DAG is being deleted, remove it from the database
		if err := r.deleteFromDatabase(ctx, req.NamespacedName); err != nil {
			return ctrl.Result{}, err
		}
		// Remove the finalizer if it exists
		if controllerutil.ContainsFinalizer(&dag, "dag.finalizer.kontroler.greedykomodo") {
			controllerutil.RemoveFinalizer(&dag, "dag.finalizer.kontroler.greedykomodo")
			if err := r.Update(ctx, &dag); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Validate the DAG
	if err := dag.ValidateDAG(); err != nil {
		// Return error if DAG is not valid
		return ctrl.Result{}, err
	}

	// Store the DAG object in the database
	if err := r.storeInDatabase(ctx, &dag, req.NamespacedName.Namespace); err != nil {
		if err.Error() == "applying the same dag" {
			log.Log.Info("reconcile event", "controller", "dag", "event", "applying the same dag")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DAGReconciler) storeInDatabase(ctx context.Context, dag *kontrolerv1alpha1.DAG, namespace string) error {
	return r.DbManager.InsertDAG(ctx, dag, namespace)
}

func (r *DAGReconciler) deleteFromDatabase(ctx context.Context, namespacedName types.NamespacedName) error {
	return r.DbManager.SoftDeleteDAG(ctx, namespacedName.Name, namespacedName.Namespace)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DAGReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kontrolerv1alpha1.DAG{}).
		Complete(r)
}

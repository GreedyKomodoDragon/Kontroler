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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/dagdsl"
	"kontroler-controller/internal/db"
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
		if errors.IsNotFound(err) {
			return r.handleDeletion(ctx, req, dag)
		}
		return ctrl.Result{}, err
	}

	// Check if the DAG is marked for deletion
	if !dag.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, req, dag)
	}

	// Process DSL if provided
	if dag.Spec.DSL != "" {
		if err := r.processDSL(ctx, &dag); err != nil {
			return r.markDAGFailed(ctx, &dag, fmt.Sprintf("failed to process DSL: %s", err.Error()))
		}

		// Update the DAG with the processed DSL content
		if err := r.Update(ctx, &dag); err != nil {
			return r.markDAGFailed(ctx, &dag, fmt.Sprintf("failed to update DAG after DSL processing: %s", err.Error()))
		}
	}

	taskRefs := []kontrolerv1alpha1.TaskRef{}
	for _, val := range dag.Spec.Task {
		if val.TaskRef != nil {
			if val.TaskRef.Name == "" || val.TaskRef.Version == 0 {
				return r.markDAGFailed(ctx, &dag, "missing name or version")
			}
			taskRefs = append(taskRefs, *val.TaskRef)
		}
	}

	refParams, err := r.DbManager.GetTaskRefsParameters(ctx, taskRefs)
	if err != nil {
		return r.markDAGFailed(ctx, &dag, fmt.Sprintf("failed at GetTaskRefsParameters: %s", err.Error()))
	}

	// Validate the DAG
	if err := dag.ValidateDAG(refParams); err != nil {
		return r.markDAGFailed(ctx, &dag, fmt.Sprintf("failed to validate dag: %s", err.Error()))
	}

	// Store the DAG object in the database
	if err := r.storeInDatabase(ctx, &dag, req.NamespacedName.Namespace); err != nil {
		if err.Error() == "applying the same dag" {
			log.Log.Info("reconcile event", "controller", "dag", "event", "applying the same dag")
			return ctrl.Result{}, nil
		}
		return r.markDAGFailed(ctx, &dag, fmt.Sprintf("failed to store dag in db: %s", err.Error()))
	}

	r.updatingDagTaskFinalisers(ctx, taskRefs, req.Namespace)

	return r.markDAGSuccessful(ctx, &dag)
}

func (r *DAGReconciler) markDAGFailed(ctx context.Context, dag *kontrolerv1alpha1.DAG, reason string) (ctrl.Result, error) {
	dag.Status.Phase = "Failed"
	dag.Status.Message = reason
	if err := r.Status().Update(ctx, dag); err != nil {
		log.Log.Error(err, "failed to update DAG status", "dag", dag.Name)
		return ctrl.Result{}, err
	}
	log.Log.Info("DAG marked as failed", "dag", dag.Name, "reason", reason)
	return ctrl.Result{}, nil
}

func (r *DAGReconciler) markDAGSuccessful(ctx context.Context, dag *kontrolerv1alpha1.DAG) (ctrl.Result, error) {
	dag.Status.Phase = "Successful"
	dag.Status.Message = "DAG reconciled successfully"
	if err := r.Status().Update(ctx, dag); err != nil {
		log.Log.Error(err, "failed to update DAG status", "dag", dag.Name)
		return ctrl.Result{}, err
	}
	log.Log.Info("DAG marked as successful", "dag", dag.Name)
	return ctrl.Result{}, nil
}

func (r *DAGReconciler) storeInDatabase(ctx context.Context, dag *kontrolerv1alpha1.DAG, namespace string) error {
	return r.DbManager.InsertDAG(ctx, dag, namespace)
}

func (r *DAGReconciler) deleteFromDatabase(ctx context.Context, namespacedName types.NamespacedName) ([]string, error) {
	log.Log.Info("reconcile deletion", "controller", "dag", "namespacedName.Name", namespacedName.Name, "namespacedName.Namespace", namespacedName.Namespace)
	return r.DbManager.DeleteDAG(ctx, namespacedName.Name, namespacedName.Namespace)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DAGReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kontrolerv1alpha1.DAG{}).
		Complete(r)
}

func (r *DAGReconciler) updatingDagTaskFinalisers(ctx context.Context, taskRefs []kontrolerv1alpha1.TaskRef, namespace string) {
	for _, taskRef := range taskRefs {
		var dagTask kontrolerv1alpha1.DagTask
		if err := r.Get(ctx, types.NamespacedName{
			Name:      taskRef.Name,
			Namespace: namespace,
		}, &dagTask); err != nil {
			// Log the error and continue
			log.Log.Error(err, "failed to fetch dagTask", "taskRef", taskRef)
			continue
		}

		if !controllerutil.ContainsFinalizer(&dagTask, "dagTask.finalizer.kontroler.greedykomodo") {
			if updated := controllerutil.AddFinalizer(&dagTask, "dagTask.finalizer.kontroler.greedykomodo"); !updated {
				log.Log.Error(fmt.Errorf("AddFinalizer failed"), "failed to add finalizer to dagTask", "taskRef", taskRef)
				continue
			}

			if err := r.Update(ctx, &dagTask); err != nil {
				log.Log.Error(err, "failed to add finalizer to dagTask", "taskRef", taskRef)
			}
		}
	}
}

func (r *DAGReconciler) removingDagTaskFinalisers(ctx context.Context, taskRefs []string, namespace string) {
	log.Log.Info("reconcile deletion", "controller", "dag", "namespace", namespace, "method", "removingDagTaskFinalisers", "taskCount", len(taskRefs))

	for _, taskRef := range taskRefs {
		var dagTask kontrolerv1alpha1.DagTask
		if err := r.Get(ctx, types.NamespacedName{
			Name:      taskRef,
			Namespace: namespace,
		}, &dagTask); err != nil {
			// Log the error and continue
			log.Log.Error(err, "failed to fetch dagTask", "taskRef", taskRef)
			continue
		}

		if controllerutil.ContainsFinalizer(&dagTask, "dagTask.finalizer.kontroler.greedykomodo") {

			if updated := controllerutil.RemoveFinalizer(&dagTask, "dagTask.finalizer.kontroler.greedykomodo"); !updated {
				log.Log.Error(fmt.Errorf("RemoveFinalizer failed"), "failed to remove finalizer to dagTask", "taskRef", taskRef)
				continue
			}

			if err := r.Update(ctx, &dagTask); err != nil {
				log.Log.Error(err, "failed to remove finalizer from dagTask", "taskRef", taskRef)
			}
		}
	}
}

func (r *DAGReconciler) handleDeletion(ctx context.Context, req ctrl.Request, dag kontrolerv1alpha1.DAG) (ctrl.Result, error) {
	// The DAG is being deleted, remove it from the database
	taskNames, err := r.deleteFromDatabase(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Remove the finalizer if it exists
	if controllerutil.ContainsFinalizer(&dag, "dag.finalizer.kontroler.greedykomodo") {
		controllerutil.RemoveFinalizer(&dag, "dag.finalizer.kontroler.greedykomodo")
		if err := r.Update(ctx, &dag); err != nil {
			log.Log.Error(err, "failed to remove finalizer from dag", "dag", dag.Name)
			return ctrl.Result{}, err
		}
	}

	r.removingDagTaskFinalisers(ctx, taskNames, req.Namespace)

	return ctrl.Result{}, nil
}

// processDSL parses the DSL string and populates the DAG spec fields
func (r *DAGReconciler) processDSL(ctx context.Context, dag *kontrolerv1alpha1.DAG) error {
	// Parse the DSL string
	parsedSpec, err := dagdsl.ParseDSL(dag.Spec.DSL)
	if err != nil {
		return fmt.Errorf("failed to parse DSL: %w", err)
	}

	// Validate the parsed DSL
	validationResult := dagdsl.ValidateDAGSpec(parsedSpec)
	if !validationResult.Valid {
		errorMsg := "DSL validation failed:"
		for _, validationError := range validationResult.Errors {
			errorMsg += fmt.Sprintf(" %s", validationError.Error())
		}
		return fmt.Errorf(errorMsg)
	}

	// Merge the parsed DSL content with the existing spec
	// DSL takes precedence, but we preserve fields that aren't defined in DSL
	if parsedSpec.Schedule != "" {
		dag.Spec.Schedule = parsedSpec.Schedule
	}

	if len(parsedSpec.Parameters) > 0 {
		dag.Spec.Parameters = parsedSpec.Parameters
	}

	if len(parsedSpec.Task) > 0 {
		dag.Spec.Task = parsedSpec.Task
	}

	log.Log.Info("DSL processed successfully", "dag", dag.Name, "taskCount", len(parsedSpec.Task))
	return nil
}

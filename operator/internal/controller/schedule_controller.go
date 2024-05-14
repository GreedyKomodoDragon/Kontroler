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

	kubeconductorv1alpha1 "github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
)

// ScheduleReconciler reconciles a Schedule object
type ScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kubeconductor.greedykomodo,resources=schedules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubeconductor.greedykomodo,resources=schedules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubeconductor.greedykomodo,resources=schedules/finalizers,verbs=update

func (r *ScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// Fetch the Schedule object that triggered the reconciliation
	var schedule kubeconductorv1alpha1.Schedule
	if err := r.Get(ctx, req.NamespacedName, &schedule); err != nil {
		// Handle case where Schedule object is not found
		if errors.IsNotFound(err) {
			// Schedule object was deleted, delete it from the database if present
			if err := r.deleteScheduleFromDatabase(req.NamespacedName); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		// Return error if unable to fetch Schedule object
		return ctrl.Result{}, err
	}

	// Schedule object was found, store it in the database
	if err := r.storeScheduleInDatabase(&schedule); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ScheduleReconciler) storeScheduleInDatabase(schedule *kubeconductorv1alpha1.Schedule) error {
	fmt.Println("storing in the database")
	return nil
}

func (r *ScheduleReconciler) deleteScheduleFromDatabase(name types.NamespacedName) error {
	fmt.Println("delete the database")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeconductorv1alpha1.Schedule{}).
		Complete(r)
}

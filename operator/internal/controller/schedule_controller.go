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

	kubeconductorv1alpha1 "github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
)

// ScheduleReconciler reconciles a Schedule object
type ScheduleReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	DbManager db.DbManager
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
			if err := r.deleteScheduleFromDatabase(schedule.UID); err != nil {
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
	// Conditional is optional so we want to keep this blank if not enabled
	retryCodes := []int32{}
	if schedule.Spec.Conditional.Enabled {
		retryCodes = schedule.Spec.Conditional.RetryCodes
	}

	return r.DbManager.UpsertCronJob(context.Background(), &db.CronJob{
		Id:           schedule.UID,
		Schedule:     schedule.Spec.CronSchedule,
		ImageName:    schedule.Spec.ImageName,
		Command:      schedule.Spec.Command,
		Args:         schedule.Spec.Args,
		BackoffLimit: schedule.Spec.BackoffLimit,
		ConditionalRetry: db.ConditionalRetry{
			Enabled:    schedule.Spec.Conditional.Enabled,
			RetryCodes: retryCodes,
		},
	})
}

func (r *ScheduleReconciler) deleteScheduleFromDatabase(uid types.UID) error {
	return r.DbManager.DeleteCronJob(context.Background(), uid)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeconductorv1alpha1.Schedule{}).
		Complete(r)
}

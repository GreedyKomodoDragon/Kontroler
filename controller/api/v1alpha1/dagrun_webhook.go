package v1alpha1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var dagrunlog = logf.Log.WithName("dagrun-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *DagRun) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-kontroler-greedykomodo-v1alpha1-dagrun,mutating=true,failurePolicy=fail,sideEffects=None,groups=kontroler.greedykomodo,resources=dagruns,verbs=create;update,versions=v1alpha1,name=mdagrun.kb.io,admissionReviewVersions=v1

var _ admission.CustomDefaulter = &DagRun{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DagRun) Default(ctx context.Context, obj runtime.Object) error {
	dagRun, ok := obj.(*DagRun)
	if !ok {
		return fmt.Errorf("expected *DagRun, got %T", obj)
	}
	dagrunlog.Info("default", "name", dagRun.Name)
	return nil
}

//+kubebuilder:webhook:path=/validate-kontroler-greedykomodo-v1alpha1-dagrun,mutating=false,failurePolicy=fail,sideEffects=None,groups=kontroler.greedykomodo,resources=dagruns,verbs=create;update,versions=v1alpha1,name=vdagrun.kb.io,admissionReviewVersions=v1

var _ admission.CustomValidator = &DagRun{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DagRun) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dagRun, ok := obj.(*DagRun)
	if !ok {
		return nil, fmt.Errorf("expected *DagRun, got %T", obj)
	}
	return dagRun.validateDagRun()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DagRun) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	dagRun, ok := newObj.(*DagRun)
	if !ok {
		return nil, fmt.Errorf("expected *DagRun, got %T", newObj)
	}
	return dagRun.validateDagRun()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DagRun) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dagRun, ok := obj.(*DagRun)
	if !ok {
		return nil, fmt.Errorf("expected *DagRun, got %T", obj)
	}
	dagrunlog.Info("validate delete", "name", dagRun.Name)
	return nil, nil
}

// validateDagRun is a helper function to validate DagRun creation and update.
func (r *DagRun) validateDagRun() (admission.Warnings, error) {
	if r.Spec.DagName == "" {
		return nil, errors.New("DagName cannot be empty")
	}

	for _, param := range r.Spec.Parameters {
		if param.Name == "" {
			return nil, fmt.Errorf("parameter name must be set")
		}
		if param.Value != "" && param.FromSecret != "" {
			return nil, fmt.Errorf("only one of value or fromSecret can be set for parameter %s", param.Name)
		}
		if param.Value == "" && param.FromSecret == "" {
			return nil, fmt.Errorf("either value or fromSecret must be set for parameter %s", param.Name)
		}
	}
	return nil, nil
}

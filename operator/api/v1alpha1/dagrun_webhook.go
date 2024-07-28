package v1alpha1

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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

//+kubebuilder:webhook:path=/mutate-kubeconductor-greedykomodo-v1alpha1-dagrun,mutating=true,failurePolicy=fail,sideEffects=None,groups=kubeconductor.greedykomodo,resources=dagruns,verbs=create;update,versions=v1alpha1,name=mdagrun.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &DagRun{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DagRun) Default() {
	dagrunlog.Info("default", "name", r.Name)

	// TODO: fill in your defaulting logic.
}

//+kubebuilder:webhook:path=/validate-kubeconductor-greedykomodo-v1alpha1-dagrun,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubeconductor.greedykomodo,resources=dagruns,verbs=create;update,versions=v1alpha1,name=vdagrun.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &DagRun{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DagRun) ValidateCreate() (admission.Warnings, error) {
	dagrunlog.Info("validate create", "name", r.Name)
	return r.validateDagRun()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DagRun) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	dagrunlog.Info("validate update", "name", r.Name)
	return r.validateDagRun()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DagRun) ValidateDelete() (admission.Warnings, error) {
	dagrunlog.Info("validate delete", "name", r.Name)
	// No validation logic needed for deletion.
	return nil, nil
}

// validateDagRun is a helper function to validate DagRun creation and update.
func (r *DagRun) validateDagRun() (admission.Warnings, error) {
	if r.Spec.DagId == 0 {
		return nil, errors.New("DagId must be set")
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

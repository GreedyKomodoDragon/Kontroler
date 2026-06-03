package controller

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
)

// ensureWorkerRBAC creates a ServiceAccount, Role and RoleBinding in the WorkerPool namespace
// and returns the name of the ServiceAccount to use. If a ServiceAccount already exists it is left intact.
func (r *WorkerPoolReconciler) ensureWorkerRBAC(ctx context.Context, wp *kontrolerv1alpha1.WorkerPool) (string, error) {
	// choose SA name
	saName := wp.Name + "-sa"
	roleName := wp.Name + "-pod-ops"
	rbName := wp.Name + "-pod-ops-binding"

	// create or update ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: wp.Namespace},
	}
	if err := controllerutil.SetControllerReference(wp, sa, r.Scheme); err != nil {
		return "", err
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error { return nil }); err != nil {
		return "", fmt.Errorf("failed to create or update serviceaccount: %w", err)
	}

	// create or update Role with pod permissions (namespace-scoped)
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: wp.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods/exec"},
				Verbs:     []string{"create"},
			},
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to create or update role: %w", err)
	}

	// create or update RoleBinding to bind role -> serviceaccount
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: wp.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: saName, Namespace: wp.Namespace}}
		rb.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: roleName}
		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to create or update rolebinding: %w", err)
	}

	return saName, nil
}

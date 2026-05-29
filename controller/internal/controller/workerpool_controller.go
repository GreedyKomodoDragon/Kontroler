package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkerPoolReconciler reconciles a WorkerPool object
type WorkerPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=workerpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kontroler.greedykomodo,resources=workerpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *WorkerPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("workerpool-reconciler").WithValues("workerpool", req.NamespacedName)

	var wp kontrolerv1alpha1.WorkerPool
	if err := r.Get(ctx, req.NamespacedName, &wp); err != nil {
		// resource not found or other error
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// default replicas
	replicas := int32(1)
	if wp.Spec.Replicas != nil {
		replicas = *wp.Spec.Replicas
	}

	// construct desired Deployment
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wp.Name + "-workers",
			Namespace: wp.Namespace,
			Labels: map[string]string{
				"app":        "kontroler-worker",
				"workerpool": wp.Name,
			},
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		// set owner
		if err := controllerutil.SetControllerReference(&wp, dep, r.Scheme); err != nil {
			return err
		}

		dep.Spec.Replicas = &replicas
		// Pod template
		var containerImage string
		if wp.Spec.Image != "" {
			containerImage = wp.Spec.Image
		} else {
			containerImage = "greedykomodo/kontroler-worker:latest"
		}

		// default envs and mapping
		envs := []corev1.EnvVar{
			{Name: "WORKERPOOL_NAME", Value: wp.Name},
		}

		if wp.Spec.Concurrency != nil {
			if wp.Spec.Concurrency.ClaimBatchSize != nil {
				envs = append(envs, corev1.EnvVar{Name: "CLAIM_BATCH_SIZE", Value: fmtIntPtr(wp.Spec.Concurrency.ClaimBatchSize)})
			}
			if wp.Spec.Concurrency.MaxConcurrentClaims != nil {
				envs = append(envs, corev1.EnvVar{Name: "MAX_CONCURRENT_CLAIMS", Value: fmtIntPtr(wp.Spec.Concurrency.MaxConcurrentClaims)})
			}
		}

		if wp.Spec.Lease != nil && wp.Spec.Lease.TTLSeconds != nil {
			envs = append(envs, corev1.EnvVar{Name: "DEFAULT_LEASE_TTL_SECONDS", Value: fmtIntPtr(wp.Spec.Lease.TTLSeconds)})
		}

		// mount DB secret name as env var reference if provided
		if wp.Spec.DBSecretRef != "" {
			envs = append(envs, corev1.EnvVar{Name: "DB_SECRET_NAME", Value: wp.Spec.DBSecretRef})
		}

		// build container spec
		dep.Spec.Template.ObjectMeta = metav1.ObjectMeta{Labels: dep.Labels}
		dep.Spec.Template.Spec = corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "worker",
					Image: containerImage,
					Env:   envs,
					Ports: []corev1.ContainerPort{{ContainerPort: 9100, Name: "metrics"}},
					ReadinessProbe: &corev1.Probe{
						Handler:             corev1.Handler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstrFromInt(8080)}},
						InitialDelaySeconds: 5,
						PeriodSeconds:       10,
					},
				},
			},
		}

		// merge PodTemplate from spec if provided (only a few fields for now)
		if wp.Spec.PodTemplate != nil {
			if wp.Spec.PodTemplate.NodeSelector != nil {
				dep.Spec.Template.Spec.NodeSelector = wp.Spec.PodTemplate.NodeSelector
			}
			if wp.Spec.PodTemplate.ServiceAccountName != "" {
				dep.Spec.Template.Spec.ServiceAccountName = wp.Spec.PodTemplate.ServiceAccountName
			}
			// TODO: merge tolerations, resources, affinity if needed
		}

		return nil
	})

	if err != nil {
		log.Error(err, "failed to create or update deployment")
		return ctrl.Result{}, err
	}

	// update status
	wp.Status.Replicas = replicas
	// fetch ready replicas
	var curr appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, &curr); err == nil {
		wp.Status.ReadyReplicas = curr.Status.ReadyReplicas
		t := metav1.NewTime(time.Now())
		wp.Status.LastReconcileTime = &t
		_ = r.Status().Update(ctx, &wp)
	}

	log.Info("reconciled workerpool", "name", wp.Name, "result", opResult)
	return ctrl.Result{}, nil
}

func (r *WorkerPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kontrolerv1alpha1.WorkerPool{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// helper functions (small adapters)
func fmtIntPtr(p *int32) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}

// intstrFromInt returns an IntOrString for a numeric port
func intstrFromInt(i int) metav1.IntOrString {
	return metav1.IntOrString{Type: metav1.Int, IntVal: int32(i)}
}

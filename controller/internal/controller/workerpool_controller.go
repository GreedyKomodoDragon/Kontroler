package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

		// merge PodTemplate from spec if provided
		if wp.Spec.PodTemplate != nil {
			pt := wp.Spec.PodTemplate

			// node selector
			if pt.NodeSelector != nil {
				dep.Spec.Template.Spec.NodeSelector = pt.NodeSelector
			}

			// service account
			if pt.ServiceAccountName != "" {
				dep.Spec.Template.Spec.ServiceAccountName = pt.ServiceAccountName
			}

			// automount service account token
			if pt.AutomountServiceAccountToken != nil {
				a := *pt.AutomountServiceAccountToken
				dep.Spec.Template.Spec.AutomountServiceAccountToken = &a
			}

			// active deadline seconds
			if pt.ActiveDeadlineSeconds != nil {
				dep.Spec.Template.Spec.ActiveDeadlineSeconds = pt.ActiveDeadlineSeconds
			}

			// image pull secrets
			if len(pt.ImagePullSecrets) > 0 {
				ips := make([]corev1.LocalObjectReference, 0, len(pt.ImagePullSecrets))
				for _, s := range pt.ImagePullSecrets {
					ips = append(ips, corev1.LocalObjectReference{Name: s.Name})
				}
				dep.Spec.Template.Spec.ImagePullSecrets = ips
			}

			// tolerations
			if len(pt.Tolerations) > 0 {
				tols := make([]corev1.Toleration, 0, len(pt.Tolerations))
				for _, t := range pt.Tolerations {
					tols = append(tols, corev1.Toleration{
						Key:               t.Key,
						Operator:          corev1.TolerationOperator(t.Operator),
						Value:             t.Value,
						Effect:            corev1.TaintEffect(t.Effect),
						TolerationSeconds: t.TolerationSeconds,
					})
				}
				dep.Spec.Template.Spec.Tolerations = tols
			}

			// volumes
			if len(pt.Volumes) > 0 {
				vols := dep.Spec.Template.Spec.Volumes
				for _, v := range pt.Volumes {
					vol := corev1.Volume{Name: v.Name}
					if v.EmptyDir != nil {
						vol.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
					} else if v.PersistentVolumeClaim != nil {
						vol.VolumeSource = corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: v.PersistentVolumeClaim.ClaimName}}
					}
					vols = append(vols, vol)
				}
				dep.Spec.Template.Spec.Volumes = vols
			}

			// container volume mounts
			if len(pt.VolumeMounts) > 0 {
				mounts := dep.Spec.Template.Spec.Containers[0].VolumeMounts
				for _, vm := range pt.VolumeMounts {
					mounts = append(mounts, corev1.VolumeMount{Name: vm.Name, MountPath: vm.MountPath, ReadOnly: vm.ReadOnly})
				}
				dep.Spec.Template.Spec.Containers[0].VolumeMounts = mounts
			}

			// security context (only FSGroup supported)
			if pt.SecurityContext != nil {
				psc := &corev1.PodSecurityContext{}
				if pt.SecurityContext.FSGroup != nil {
					psc.FSGroup = pt.SecurityContext.FSGroup
				}
				dep.Spec.Template.Spec.SecurityContext = psc
			}

			// resources for the first container if provided
			if pt.Resources != nil {
				req := corev1.ResourceList{}
				lim := corev1.ResourceList{}
				for k, v := range pt.Resources.Requests {
					if r, err := resource.ParseQuantity(v); err == nil {
						req[corev1.ResourceName(k)] = r
					}
				}
				for k, v := range pt.Resources.Limits {
					if r, err := resource.ParseQuantity(v); err == nil {
						lim[corev1.ResourceName(k)] = r
					}
				}
				if dep.Spec.Template.Spec.Containers[0].Resources.Requests == nil {
					dep.Spec.Template.Spec.Containers[0].Resources.Requests = req
				} else {
					for k, q := range req {
						dep.Spec.Template.Spec.Containers[0].Resources.Requests[k] = q
					}
				}
				if dep.Spec.Template.Spec.Containers[0].Resources.Limits == nil {
					dep.Spec.Template.Spec.Containers[0].Resources.Limits = lim
				} else {
					for k, q := range lim {
						dep.Spec.Template.Spec.Containers[0].Resources.Limits[k] = q
					}
				}
			}

			// affinity: provided as raw JSON; attempt to unmarshal into corev1.Affinity parts
			if pt.Affinity != nil {
				var af corev1.Affinity
				// nodeAffinity
				if pt.Affinity.NodeAffinity != nil {
					_ = json.Unmarshal(pt.Affinity.NodeAffinity.Raw, &af.NodeAffinity)
				}
				if pt.Affinity.PodAffinity != nil {
					_ = json.Unmarshal(pt.Affinity.PodAffinity.Raw, &af.PodAffinity)
				}
				if pt.Affinity.PodAntiAffinity != nil {
					_ = json.Unmarshal(pt.Affinity.PodAntiAffinity.Raw, &af.PodAntiAffinity)
				}
				dep.Spec.Template.Spec.Affinity = &af
			}
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

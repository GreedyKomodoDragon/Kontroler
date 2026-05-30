package controller

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("WorkerPool Controller", func() {
	Context("When reconciling a WorkerPool resource", func() {
		const resourceName = "test-workerpool"
		ctx := context.Background()
		nn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		BeforeEach(func() {
			wp := &kontrolerv1alpha1.WorkerPool{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
				Spec: kontrolerv1alpha1.WorkerPoolSpec{
					Replicas: ptrInt32(1),
					Image:    "busybox:latest",
					Concurrency: &struct {
						MaxConcurrentClaims *int32 "json:\"maxConcurrentClaims,omitempty\""
						ClaimBatchSize      *int32 "json:\"claimBatchSize,omitempty\""
					}{
						MaxConcurrentClaims: ptrInt32(5),
						ClaimBatchSize:      ptrInt32(2),
					},
				},
			}
			Expect(k8sClient.Create(ctx, wp)).To(Succeed())
		})

		AfterEach(func() {
			wp := &kontrolerv1alpha1.WorkerPool{}
			_ = k8sClient.Get(ctx, nn, wp)
			_ = k8sClient.Delete(ctx, wp)
		})

		It("should create a Deployment for the WorkerPool", func() {
			reconciler := &WorkerPoolReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			// give controller-runtime fake client a moment to persist
			// then retrieve Deployment
			dep := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-workers", Namespace: "default"}, dep)
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
			Expect(dep.Spec.Replicas).NotTo(BeNil())
			Expect(*dep.Spec.Replicas).To(Equal(int32(1)))
		})

		It("should merge PodTemplate into generated Deployment", func() {
			// create a WorkerPool with rich PodTemplate
			n := "test-podtemplate"
			nn2 := types.NamespacedName{Name: n, Namespace: "default"}
			nodeSel := map[string]string{"node-role.kubernetes.io/worker": "true"}
			ips := []kontrolerv1alpha1.LocalObjectReference{{Name: "my-pull-secret"}}
			mountName := "workspace"
			volumes := []kontrolerv1alpha1.Volume{{Name: mountName, PersistentVolumeClaim: &kontrolerv1alpha1.PersistentVolumeClaimVolumeSource{ClaimName: "my-claim"}}}
			vms := []kontrolerv1alpha1.VolumeMount{{Name: mountName, MountPath: "/workspace", ReadOnly: false}}
			fs := int64(1000)
			resources := &kontrolerv1alpha1.ResourceRequirements{
				Requests: map[string]string{"cpu": "100m", "memory": "128Mi"},
				Limits:   map[string]string{"cpu": "500m", "memory": "512Mi"},
			}

			// build an affinity JSON
			aff := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{},
			}
			affJSON, _ := json.Marshal(aff)
			affWrapper := &kontrolerv1alpha1.Affinity{
				NodeAffinity: &apiextensionsv1.JSON{Raw: affJSON},
			}

			wp := &kontrolerv1alpha1.WorkerPool{
				ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: "default"},
				Spec: kontrolerv1alpha1.WorkerPoolSpec{
					Replicas: ptrInt32(1),
					Image:    "busybox:latest",
					PodTemplate: &kontrolerv1alpha1.PodTemplateSpec{
						NodeSelector:                 nodeSel,
						ServiceAccountName:           "sa-name",
						AutomountServiceAccountToken: ptrBool(true),
						ActiveDeadlineSeconds:        ptrInt64(120),
						ImagePullSecrets:             ips,
						Tolerations:                  []kontrolerv1alpha1.Toleration{{Key: "k1", Operator: "Equal", Value: "v1", Effect: "NoSchedule"}},
						Volumes:                      volumes,
						VolumeMounts:                 vms,
						SecurityContext:              &kontrolerv1alpha1.PodSecurityContext{FSGroup: &fs},
						Resources:                    resources,
						Affinity:                     affWrapper,
					},
				},
			}

			Expect(k8sClient.Create(ctx, wp)).To(Succeed())

			reconciler := &WorkerPoolReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nn2})
			Expect(err).NotTo(HaveOccurred())

			dep := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: n + "-workers", Namespace: "default"}, dep)
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())

			// assertions
			Expect(dep.Spec.Template.Spec.NodeSelector).To(Equal(nodeSel))
			Expect(dep.Spec.Template.Spec.ServiceAccountName).To(Equal("sa-name"))
			Expect(*dep.Spec.Template.Spec.AutomountServiceAccountToken).To(BeTrue())
			Expect(dep.Spec.Template.Spec.ActiveDeadlineSeconds).NotTo(BeNil())
			Expect(*dep.Spec.Template.Spec.ActiveDeadlineSeconds).To(Equal(int64(120)))
			// image pull secret
			Expect(len(dep.Spec.Template.Spec.ImagePullSecrets)).To(Equal(1))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("my-pull-secret"))
			// tolerations
			Expect(len(dep.Spec.Template.Spec.Tolerations)).To(Equal(1))
			Expect(dep.Spec.Template.Spec.Tolerations[0].Key).To(Equal("k1"))
			// volumes
			found := false
			for _, v := range dep.Spec.Template.Spec.Volumes {
				if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == "my-claim" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
			// volume mounts
			Expect(len(dep.Spec.Template.Spec.Containers[0].VolumeMounts)).To(BeNumerically(">=", 1))
			// security context
			Expect(dep.Spec.Template.Spec.SecurityContext).NotTo(BeNil())
			Expect(dep.Spec.Template.Spec.SecurityContext.FSGroup).NotTo(BeNil())
			Expect(*dep.Spec.Template.Spec.SecurityContext.FSGroup).To(Equal(int64(1000)))
			// resources
			reqs := dep.Spec.Template.Spec.Containers[0].Resources.Requests
			Expect(reqs[corev1.ResourceCPU].String()).To(Equal("100m"))
			Expect(reqs[corev1.ResourceMemory].String()).To(Equal("128Mi"))
			// affinity presence
			Expect(dep.Spec.Template.Spec.Affinity).NotTo(BeNil())
		})

		It("should scale down and remove finalizer on deletion", func() {
			// create WP
			n := "test-delete"
			nn3 := types.NamespacedName{Name: n, Namespace: "default"}
			wp := &kontrolerv1alpha1.WorkerPool{
				ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: "default"},
				Spec: kontrolerv1alpha1.WorkerPoolSpec{
					Replicas:                ptrInt32(1),
					GracefulShutdownSeconds: ptrInt32(30),
				},
			}
			Expect(k8sClient.Create(ctx, wp)).To(Succeed())

			reconciler := &WorkerPoolReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nn3})
			Expect(err).NotTo(HaveOccurred())

			// ensure deployment exists
			dep := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: n + "-workers", Namespace: "default"}, dep)
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())

			// simulate ready replicas present
			dep.Status.ReadyReplicas = 1
			Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

			// delete the WorkerPool (sets deletionTimestamp)
			Expect(k8sClient.Delete(ctx, wp)).To(Succeed())

			// first reconcile should scale deployment to 0
			_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nn3})
			Expect(err).NotTo(HaveOccurred())

			// fetch deployment and assert replicas = 0
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: n + "-workers", Namespace: "default"}, dep)).To(Succeed())
			Expect(dep.Spec.Replicas).NotTo(BeNil())
			Expect(*dep.Spec.Replicas).To(Equal(int32(0)))

			// still has readyReplicas, simulate pods terminated
			dep.Status.ReadyReplicas = 0
			Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())

			// second reconcile should remove finalizer and allow deletion
			_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nn3})
			Expect(err).NotTo(HaveOccurred())

			// object should be deleted (not found)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, nn3, &kontrolerv1alpha1.WorkerPool{})
				return apierrors.IsNotFound(err)
			}, 5*time.Second, 250*time.Millisecond).Should(BeTrue())
		})
	})
})

func ptrInt32(i int32) *int32 { return &i }

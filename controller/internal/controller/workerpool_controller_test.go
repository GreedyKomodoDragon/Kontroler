package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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
	})
})

func ptrInt32(i int32) *int32 { return &i }

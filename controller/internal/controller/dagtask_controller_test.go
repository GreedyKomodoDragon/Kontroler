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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"

	cron "github.com/robfig/cron/v3"
)

var _ = Describe("DagTask Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		dagtask := &kontrolerv1alpha1.DagTask{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind DagTask")
			err := k8sClient.Get(ctx, typeNamespacedName, dagtask)
			if err != nil && errors.IsNotFound(err) {
				resource := &kontrolerv1alpha1.DagTask{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: kontrolerv1alpha1.DagTaskSpec{
						Image:   "alpine:latest",
						Command: []string{"echo"},
						Args:    []string{"Hello World"},
						Backoff: kontrolerv1alpha1.Backoff{
							Limit: 3,
						},
						Conditional: kontrolerv1alpha1.Conditional{
							Enabled:    false,
							RetryCodes: []int{1, 2},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &kontrolerv1alpha1.DagTask{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance DagTask")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			// Create in-memory SQLite for testing
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			config := &db.SQLiteConfig{
				DBPath:      ":memory:",
				JournalMode: "WAL",
				Synchronous: "NORMAL",
				CacheSize:   -2000,
				TempStore:   "MEMORY",
			}
			dbManager, _, err := db.NewSqliteManager(context.TODO(), &parser, config)
			Expect(err).NotTo(HaveOccurred())

			// Initialize the database
			err = dbManager.InitaliseDatabase(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			controllerReconciler := &DagTaskReconciler{
				Client:    k8sClient,
				Scheme:    k8sClient.Scheme(),
				DbManager: dbManager,
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})

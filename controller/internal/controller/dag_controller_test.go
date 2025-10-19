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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"

	cron "github.com/robfig/cron/v3"
)

const (
	testImage = "alpine:latest"
)

var _ = Describe("DAG Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		dag := &kontrolerv1alpha1.DAG{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind DAG")
			err := k8sClient.Get(ctx, typeNamespacedName, dag)
			if err != nil && errors.IsNotFound(err) {
				resource := &kontrolerv1alpha1.DAG{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: kontrolerv1alpha1.DAGSpec{
						Schedule: "*/5 * * * *",
						Workspace: kontrolerv1alpha1.Workspace{
							Enabled: true,
							PvcSpec: kontrolerv1alpha1.PVC{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: apiresource.MustParse("1Gi"),
									},
								},
							},
						},
						Task: []kontrolerv1alpha1.TaskSpec{
							{
								Name:    "sample-task",
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
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &kontrolerv1alpha1.DAG{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance DAG")
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

			controllerReconciler := &DAGReconciler{
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

	Context("When reconciling a DAG with DSL", func() {
		const dslResourceName = "sample-dsl-dag"

		ctx := context.Background()

		dslTypeNamespacedName := types.NamespacedName{
			Name:      dslResourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a comprehensive DAG resource with DSL")
			dslDAG := &kontrolerv1alpha1.DAG{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dslResourceName,
					Namespace: "default",
				},
				Spec: kontrolerv1alpha1.DAGSpec{
					Workspace: kontrolerv1alpha1.Workspace{
						Enabled: true,
						PvcSpec: kontrolerv1alpha1.PVC{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: apiresource.MustParse("1Gi"),
								},
							},
						},
					},
					DSL: `schedule "0 */6 * * *"

parameters {
  environment {
    default "production"
  }
  replicas {
    default "3"
  }
}

graph {
  setup -> deploy
  setup -> test
  deploy -> cleanup
  test -> cleanup
}

task setup {
  image "` + testImage + `"
  command ["sh", "-c"]
  args ["echo 'Setting up environment'"]
  parameters ["environment"]
}

task deploy {
  image "` + testImage + `"
  script """
  echo "Deploying application to $ENVIRONMENT"
  echo "Using $REPLICAS replicas"
  """
  parameters ["environment", "replicas"]
}

task test {
  image "` + testImage + `"
  script "echo 'Running tests'"
  retry [1, 2, 125]
}

task cleanup {
  image "` + testImage + `"
  command ["sh", "-c"]
  args ["echo 'Cleaning up resources'"]
}`,
				},
			}
			Expect(k8sClient.Create(ctx, dslDAG)).To(Succeed())
		})

		AfterEach(func() {
			resource := &kontrolerv1alpha1.DAG{}
			err := k8sClient.Get(ctx, dslTypeNamespacedName, resource)
			if err == nil {
				By("Cleanup the DSL DAG resource")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully process complex DSL and populate DAG spec", func() {
			By("Reconciling the DSL DAG")

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

			controllerReconciler := &DAGReconciler{
				Client:    k8sClient,
				Scheme:    k8sClient.Scheme(),
				DbManager: dbManager,
			}

			// Perform full reconciliation
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dslTypeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Get the updated DAG after reconciliation
			reconciledDAG := &kontrolerv1alpha1.DAG{}
			Expect(k8sClient.Get(ctx, dslTypeNamespacedName, reconciledDAG)).To(Succeed())

			// Verify DSL is still present
			Expect(reconciledDAG.Spec.DSL).NotTo(BeEmpty())

			// Verify the DSL was processed correctly
			By("verifying schedule parsing")
			Expect(reconciledDAG.Spec.Schedule).To(Equal("0 */6 * * *"))

			By("verifying parameters parsing")
			Expect(reconciledDAG.Spec.Parameters).To(HaveLen(2))

			envParam := findParameterByName(reconciledDAG.Spec.Parameters, "environment")
			Expect(envParam).NotTo(BeNil())
			Expect(envParam.DefaultValue).To(Equal("production"))

			replicasParam := findParameterByName(reconciledDAG.Spec.Parameters, "replicas")
			Expect(replicasParam).NotTo(BeNil())
			Expect(replicasParam.DefaultValue).To(Equal("3"))

			By("verifying tasks parsing")
			Expect(reconciledDAG.Spec.Task).To(HaveLen(4))

			// Check setup task
			setupTask := findTaskByName(reconciledDAG.Spec.Task, "setup")
			Expect(setupTask).NotTo(BeNil())
			Expect(setupTask.Image).To(Equal(testImage))
			Expect(setupTask.Command).To(Equal([]string{"sh", "-c"}))
			Expect(setupTask.Args).To(Equal([]string{"echo 'Setting up environment'"}))
			Expect(setupTask.Parameters).To(Equal([]string{"environment"}))
			Expect(setupTask.RunAfter).To(BeEmpty())

			// Check deploy task
			deployTask := findTaskByName(reconciledDAG.Spec.Task, "deploy")
			Expect(deployTask).NotTo(BeNil())
			Expect(deployTask.Image).To(Equal(testImage))
			Expect(deployTask.Script).To(ContainSubstring("Deploying application"))
			Expect(deployTask.Parameters).To(Equal([]string{"environment", "replicas"}))
			Expect(deployTask.RunAfter).To(Equal([]string{"setup"}))

			// Check test task
			testTask := findTaskByName(reconciledDAG.Spec.Task, "test")
			Expect(testTask).NotTo(BeNil())
			Expect(testTask.Image).To(Equal(testImage))
			Expect(testTask.Script).To(Equal("echo 'Running tests'"))
			Expect(testTask.RunAfter).To(Equal([]string{"setup"}))
			Expect(testTask.Conditional.Enabled).To(BeTrue())
			Expect(testTask.Conditional.RetryCodes).To(Equal([]int{1, 2, 125}))

			// Check cleanup task
			cleanupTask := findTaskByName(reconciledDAG.Spec.Task, "cleanup")
			Expect(cleanupTask).NotTo(BeNil())
			Expect(cleanupTask.Image).To(Equal(testImage))
			Expect(cleanupTask.Command).To(Equal([]string{"sh", "-c"}))
			Expect(cleanupTask.Args).To(Equal([]string{"echo 'Cleaning up resources'"}))
			Expect(cleanupTask.RunAfter).To(ConsistOf("deploy", "test"))

			By("verifying DAG status is updated")
			Expect(reconciledDAG.Status.Phase).NotTo(BeEmpty())
		})

		It("should handle invalid DSL gracefully", func() {
			By("creating a DAG with invalid DSL")
			invalidDSLDAG := &kontrolerv1alpha1.DAG{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-dsl-dag",
					Namespace: "default",
				},
				Spec: kontrolerv1alpha1.DAGSpec{
					Workspace: kontrolerv1alpha1.Workspace{
						Enabled: true,
						PvcSpec: kontrolerv1alpha1.PVC{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: apiresource.MustParse("1Gi"),
								},
							},
						},
					},
					DSL: `invalid syntax here
					no proper structure`,
				},
			}
			Expect(k8sClient.Create(ctx, invalidDSLDAG)).To(Succeed())

			By("reconciling the invalid DSL DAG")

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

			controllerReconciler := &DAGReconciler{
				Client:    k8sClient,
				Scheme:    k8sClient.Scheme(),
				DbManager: dbManager,
			}

			invalidTypeNamespacedName := types.NamespacedName{
				Name:      "invalid-dsl-dag",
				Namespace: "default",
			}

			// This should handle the error gracefully
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: invalidTypeNamespacedName,
			})

			// The reconciler should not return an error but should update status
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Get the DAG and check status reflects the error
			errorDAG := &kontrolerv1alpha1.DAG{}
			Expect(k8sClient.Get(ctx, invalidTypeNamespacedName, errorDAG)).To(Succeed())
			Expect(errorDAG.Status.Phase).To(ContainSubstring("Failed"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, invalidDSLDAG)).To(Succeed())
		})

		It("should handle missing DSL fields appropriately", func() {
			By("creating a DAG with DSL missing required fields")
			partialDSLDAG := &kontrolerv1alpha1.DAG{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "partial-dsl-dag",
					Namespace: "default",
				},
				Spec: kontrolerv1alpha1.DAGSpec{
					Workspace: kontrolerv1alpha1.Workspace{
						Enabled: true,
						PvcSpec: kontrolerv1alpha1.PVC{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: apiresource.MustParse("1Gi"),
								},
							},
						},
					},
					DSL: `task lonely {
  image "` + testImage + `"
  script "echo 'I have no friends'"
}`,
				},
			}
			Expect(k8sClient.Create(ctx, partialDSLDAG)).To(Succeed())

			By("reconciling the partial DSL DAG")

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

			controllerReconciler := &DAGReconciler{
				Client:    k8sClient,
				Scheme:    k8sClient.Scheme(),
				DbManager: dbManager,
			}

			partialTypeNamespacedName := types.NamespacedName{
				Name:      "partial-dsl-dag",
				Namespace: "default",
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: partialTypeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Get the DAG and verify it processed what it could
			partialDAG := &kontrolerv1alpha1.DAG{}
			Expect(k8sClient.Get(ctx, partialTypeNamespacedName, partialDAG)).To(Succeed())

			// Should not have tasks due to validation failure (missing graph)
			Expect(partialDAG.Spec.Task).To(HaveLen(0))

			// Verify that the status reflects the validation failure
			Expect(partialDAG.Status.Phase).To(ContainSubstring("Failed"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, partialDSLDAG)).To(Succeed())
		})
	})
})

// Helper functions
func findParameterByName(params []kontrolerv1alpha1.DagParameterSpec, name string) *kontrolerv1alpha1.DagParameterSpec {
	for i, param := range params {
		if param.Name == name {
			return &params[i]
		}
	}
	return nil
}

func findTaskByName(tasks []kontrolerv1alpha1.TaskSpec, name string) *kontrolerv1alpha1.TaskSpec {
	for i, task := range tasks {
		if task.Name == name {
			return &tasks[i]
		}
	}
	return nil
}

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"

	cron "github.com/robfig/cron/v3"
)

var _ = Describe("DAG DSL hash behavior", func() {
	ctx := context.Background()

	It("writes a kontroler/dsl-hash annotation when processing DSL", func() {
		name := "dsl-hash-write"
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		dsl := `schedule "0 */6 * * *"

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
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Setting up environment'"]
  parameters ["environment"]
}

task deploy {
  image "alpine:latest"
  script """
  echo "Deploying application to $ENVIRONMENT"
  echo "Using $REPLICAS replicas"
  """
  parameters ["environment", "replicas"]
}

task test {
  image "alpine:latest"
  script "echo 'Running tests'"
  retry [1, 2, 125]
}

task cleanup {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Cleaning up resources'"]
}`

		dag := &kontrolerv1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       kontrolerv1alpha1.DAGSpec{DSL: dsl},
		}

		Expect(k8sClient.Create(ctx, dag)).To(Succeed())

		// Setup DB
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		config := &db.SQLiteConfig{DBPath: ":memory:", JournalMode: "WAL", Synchronous: "NORMAL", CacheSize: -2000, TempStore: "MEMORY"}
		dbManager, _, err := db.NewSqliteManager(context.TODO(), &parser, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(dbManager.InitaliseDatabase(context.TODO())).To(Succeed())

		reconciler := &DAGReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), DbManager: dbManager}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		reconciled := &kontrolerv1alpha1.DAG{}
		Expect(k8sClient.Get(ctx, nn, reconciled)).To(Succeed())

		hash := sha256.Sum256([]byte(reconciled.Spec.DSL))
		hs := hex.EncodeToString(hash[:])
		Expect(reconciled.Annotations).NotTo(BeNil())
		Expect(reconciled.Annotations["kontroler/dsl-hash"]).To(Equal(hs))

		Expect(k8sClient.Delete(ctx, reconciled)).To(Succeed())
	})

	It("skips processing when kontroler/dsl-hash already present", func() {
		name := "dsl-hash-skip"
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		dsl := `schedule "0 */6 * * *"

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
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Setting up environment'"]
  parameters ["environment"]
}

task deploy {
  image "alpine:latest"
  script """
  echo "Deploying application to $ENVIRONMENT"
  echo "Using $REPLICAS replicas"
  """
  parameters ["environment", "replicas"]
}

task test {
  image "alpine:latest"
  script "echo 'Running tests'"
  retry [1, 2, 125]
}

task cleanup {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Cleaning up resources'"]
}`
		hash := sha256.Sum256([]byte(dsl))
		hs := hex.EncodeToString(hash[:])

		dag := &kontrolerv1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Annotations: map[string]string{"kontroler/dsl-hash": hs}},
			Spec:       kontrolerv1alpha1.DAGSpec{DSL: dsl},
		}

		Expect(k8sClient.Create(ctx, dag)).To(Succeed())

		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		config := &db.SQLiteConfig{DBPath: ":memory:", JournalMode: "WAL", Synchronous: "NORMAL", CacheSize: -2000, TempStore: "MEMORY"}
		dbManager, _, err := db.NewSqliteManager(context.TODO(), &parser, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(dbManager.InitaliseDatabase(context.TODO())).To(Succeed())

		reconciler := &DAGReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), DbManager: dbManager}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		observed := &kontrolerv1alpha1.DAG{}
		Expect(k8sClient.Get(ctx, nn, observed)).To(Succeed())
		// schedule should remain empty as processing was skipped
		Expect(observed.Spec.Schedule).To(Equal(""))

		Expect(k8sClient.Delete(ctx, observed)).To(Succeed())
	})
})

package controller

import (
	"context"
	"testing"
	"time"

	"kontroler-controller/internal/db"

	cron "github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kfake "k8s.io/client-go/kubernetes/fake"
)

// TestRunOrphanReconciler_DeletesOrphanPods uses an in-memory sqlite manager so we don't need to implement the full interface by hand.
func TestRunOrphanReconciler_DeletesOrphanPods(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := kfake.NewSimpleClientset()

	// create a pod older than grace with kontroler/task-rid annotation pointing to missing taskRun
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orphan-pod",
			Namespace:         "default",
			Labels:            map[string]string{"managed-by": "kontroler", "kontroler/type": "task"},
			Annotations:       map[string]string{"kontroler/task-rid": "9999"},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
		},
	}

	_, err := client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}

	// create an in-memory sqlite manager
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sqliteMgr, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite manager: %v", err)
	}
	if err := sqliteMgr.InitaliseDatabase(context.Background()); err != nil {
		t.Fatalf("failed to initalise sqlite db: %v", err)
	}

	// run reconciler once with short interval
	// This function blocks until ctx.Done so run it in a goroutine and cancel after one tick
	done := make(chan struct{})
	go func() {
		_ = RunOrphanReconciler(ctx, client, sqliteMgr, 1*time.Second)
		close(done)
	}()

	// wait for a bit for reconciler to run
	time.Sleep(1500 * time.Millisecond)
	cancel()
	<-done

	_, err = client.CoreV1().Pods("default").Get(context.Background(), "orphan-pod", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected pod to be deleted, but still exists")
	}
}

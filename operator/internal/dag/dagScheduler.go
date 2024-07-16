package dag

import (
	"context"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	"github.com/google/uuid"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// DagScheduler will every min run a check on the Database to determine if a dag should be started
// For example, this could be based on a CronJob Schedule or a time window
type DagScheduler interface {
	Run()
}

type dagscheduler struct {
	dbManager     db.DBDAGManager
	dynamicClient dynamic.Interface
}

var gvr schema.GroupVersionResource = schema.GroupVersionResource{
	Group:    "kubeconductor.greedykomodo",
	Version:  "v1alpha1",
	Resource: "dagruns",
}

func NewDagScheduler(dbManager db.DBDAGManager, dynamicClient dynamic.Interface) DagScheduler {
	return &dagscheduler{
		dbManager:     dbManager,
		dynamicClient: dynamicClient,
	}
}

func (d *dagscheduler) Run() {
	tmr := time.NewTimer(time.Minute)
	for {
		<-tmr.C
		log.Log.Info("timer up, begun looking for dags to run")

		ctx := context.Background()
		dagIds, namespaces, err := d.dbManager.GetDAGsToStartAndUpdate(ctx)
		if err != nil {
			log.Log.Error(err, "failed to find dags to start")
			tmr.Reset(time.Minute)
			continue
		}

		log.Log.Info("number of dags found", "count", len(dagIds))
		opts := v1.CreateOptions{}
		for i, dagId := range dagIds {

			// Create DagRun Object Per Dag ID
			// We create a DagRun Object as it allows dagRuns to be event driven as while as scheduled
			log.Log.Info("attempting to create dagrun", "dagId", dagId)

			// Generate a unique name for each DagRun using UUID
			name := "dagrun-" + uuid.New().String()

			dagRun := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubeconductor.greedykomodo/v1alpha1",
					"kind":       "DagRun",
					"metadata": map[string]interface{}{
						"name": name,
						"labels": map[string]string{
							"app.kubernetes.io/created-by": "konductor-operator",
						},
					},
					"spec": map[string]interface{}{
						"dagId": dagId,
					},
				},
			}

			// Create the DagRun
			// TODO: Update the namespace
			if _, err := d.dynamicClient.Resource(gvr).Namespace(namespaces[i]).Create(ctx, dagRun, opts); err != nil {
				log.Log.Error(err, "failed to create DagRun", "dagId", dagId)
				continue
			}

			log.Log.Info("DagRun created successfully", "dagId", dagId, "name", name, "namespace", namespaces[i])

		}

		tmr.Reset(time.Minute)
	}

}

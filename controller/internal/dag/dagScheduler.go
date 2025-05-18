package dag

import (
	"context"
	"sync"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"

	"github.com/google/uuid"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	apiVersion     = "kontroler.greedykomodo/v1alpha1"
	kind           = "DagRun"
	createdBy      = "app.kubernetes.io/created-by"
	createdByValue = "konductor-operator"
)

// DagScheduler will every min run a check on the Database to determine if a dag should be started
// For example, this could be based on a CronJob Schedule or a time window
type DagScheduler interface {
	Run(context.Context)
}

type dagscheduler struct {
	dbManager     db.DBDAGManager
	dynamicClient dynamic.Interface
}

var gvr schema.GroupVersionResource = schema.GroupVersionResource{
	Group:    "kontroler.greedykomodo",
	Version:  "v1alpha1",
	Resource: "dagruns",
}

func NewDagScheduler(dbManager db.DBDAGManager, dynamicClient dynamic.Interface) DagScheduler {
	return &dagscheduler{
		dbManager:     dbManager,
		dynamicClient: dynamicClient,
	}
}

func (d *dagscheduler) Run(ctx context.Context) {
	tmr := time.NewTimer(time.Minute)
	defer tmr.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Log.Info("shutting down dagscheduler")
			return
		case <-tmr.C:
			go func() {
				processCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				d.processDags(processCtx)
			}()
			tmr.Reset(time.Minute)
		}
	}
}

func (d *dagscheduler) processDags(ctx context.Context) {
	log.Log.Info("timer up, begun looking for dags to run")

	dagInfos, err := d.dbManager.GetDAGsToStartAndUpdate(ctx, time.Now())
	if err != nil {
		log.Log.Error(err, "failed to find dags to start")
		return
	}

	log.Log.Info("number of dags found", "count", len(dagInfos))
	opts := v1.CreateOptions{}

	// wait group of goroutines to finish
	wg := sync.WaitGroup{}

	for _, dagInfo := range dagInfos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.createDagRun(ctx, dagInfo, opts)
		}()
	}

	wg.Wait()
}

func (d *dagscheduler) createDagRun(ctx context.Context, dagInfo *db.DagInfo, opts v1.CreateOptions) {
	log.Log.Info("attempting to create dagrun", "dagId", dagInfo.DagId)

	name := "dagrun-" + uuid.New().String()
	dagRun := d.CreateDagRunObject(dagInfo, name)

	unstructuredDagRun, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dagRun)
	if err != nil {
		log.Log.Error(err, "failed to convert DagRun to unstructured")
		return
	}

	unstructuredObj := &unstructured.Unstructured{Object: unstructuredDagRun}

	if _, err := d.dynamicClient.Resource(gvr).Namespace(dagInfo.Namespace).Create(ctx, unstructuredObj, opts); err != nil {
		log.Log.Error(err, "failed to create DagRun", "dagId", dagInfo.DagId, "name", name, "namespace", dagInfo.Namespace)
		return
	}

	log.Log.Info("DagRun created successfully", "dagId", dagInfo.DagId, "name", name, "namespace", dagInfo.Namespace)
}

// CreateDagRunObject constructs a DagRun object for the given dagInfo.
func (d *dagscheduler) CreateDagRunObject(dagInfo *db.DagInfo, name string) *v1alpha1.DagRun {
	return &v1alpha1.DagRun{
		TypeMeta: v1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: dagInfo.Namespace,
			Labels: map[string]string{
				createdBy: createdByValue,
			},
		},
		Spec: v1alpha1.DagRunSpec{
			DagName:    dagInfo.DagName,
			Parameters: []v1alpha1.ParameterSpec{},
		},
	}
}

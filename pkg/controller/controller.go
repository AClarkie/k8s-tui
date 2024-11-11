package controller

import (
	"fmt"
	"os"
	"time"

	"log/slog"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	Indexer            cache.Indexer
	Informer           cache.Controller
	deploymentClient   v1.AppsV1Interface
	logger             *slog.Logger
	queue              workqueue.TypedRateLimitingInterface[string]
	CurrentDeployments map[string]*appsv1.Deployment
}

// NewController creates a new Controller.
func NewController(coreClient v1.AppsV1Interface) *Controller {

	// Create a deployment watcher
	deploymentsListWatcher := cache.NewFilteredListWatchFromClient(coreClient.RESTClient(), "deployments", "", func(options *meta_v1.ListOptions) {})

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	indexer, informer := cache.NewIndexerInformer(deploymentsListWatcher, &appsv1.Deployment{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	return &Controller{
		Informer:           informer,
		Indexer:            indexer,
		queue:              queue,
		deploymentClient:   coreClient,
		logger:             logger,
		CurrentDeployments: make(map[string]*appsv1.Deployment),
	}
}

// Run begins watching and syncing.
func (c *Controller) Run(stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	go c.Informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.Informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	go wait.Until(c.RunWorker, time.Second, stopCh)

	<-stopCh
}

func (c *Controller) RunWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two deployments with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.syncDeployment(key)
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

// syncDeployment is the business logic of the controller. The retry logic should
// not be part of the business logic.
func (c *Controller) syncDeployment(key string) error {
	obj, exists, err := c.Indexer.GetByKey(key)
	if err != nil {
		// c.logger.Error("Fetching object from store failed", "key", key, "err", err)
		return err
	}

	if !exists {
		// c.logger.Error("deployment does not exist anymore", "key", key)
		return c.deleteDeplotment(key)
	}

	changedDeployment, err := castObjToDeployment(obj)
	if err != nil {
		return err
	}

	// TODO Business Logic
	c.CurrentDeployments[changedDeployment.GetNamespace()+"/"+changedDeployment.GetName()] = changedDeployment

	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key string) {
	if err == nil {
		// Forget about the AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// c.logger.Error("got error", "error", err)

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		// c.logger.Info("Error syncing deployment", "deployment", key, "error", err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	// c.logger.Info("Dropping deployment out of queue", "deployment", key, "error", err)
}

func (c *Controller) deleteDeplotment(key string) error {

	// TODO: Business logic here
	delete(c.CurrentDeployments, key)

	return nil
}

func castObjToDeployment(obj interface{}) (*appsv1.Deployment, error) {
	s, ok := obj.(*appsv1.Deployment)
	if !ok {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return nil, fmt.Errorf("could not cast obj to deployment, failed to create accessor, got err: %w", err)
		}
		return nil, fmt.Errorf("could not cast obj %s/%s (uid: %s) to deployment", accessor.GetNamespace(), accessor.GetName(), accessor.GetUID())
	}
	return s, nil
}

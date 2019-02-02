package controller

import (
	"fmt"
	"log"
	"time"

	utilRunTime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	ClientSet kubernetes.Interface
	Queue     workqueue.RateLimitingInterface
	Informer  cache.SharedIndexInformer
	Handler   Handler
}

type Handler interface {
	ObjectCreated(client kubernetes.Interface, obj interface{})
	ObjectDeleted(client kubernetes.Interface, obj interface{})
}

func (c *Controller) HasSynced() bool {
	return c.Informer.HasSynced()
}

func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilRunTime.HandleCrash()
	defer c.Queue.ShutDown()
	go c.Informer.Run(stopCh)

	log.Printf("Controller started at %v.\n", time.Now())
	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilRunTime.HandleError(fmt.Errorf("error syncing cache"))
		return
	}

	log.Printf("Controller synced at %v and listening for changes.\n", time.Now())
	wait.Until(c.runWorker, time.Second, stopCh)
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.Queue.Get()
	if quit {
		return false
	}

	defer c.Queue.Done(key)
	keyRaw := key.(string)

	item, exists, err := c.Informer.GetIndexer().GetByKey(keyRaw)
	if err != nil {
		log.Printf("Error retrieving by key: '%v', error: %v", key, err)
		if c.Queue.NumRequeues(key) < 5 {
			c.Queue.AddRateLimited(key)
		} else {
			log.Printf("Max retries exceeded for key: %v", key)
			c.Queue.Forget(key)
		}
		return true
	}

	if !exists {
		c.Handler.ObjectDeleted(c.ClientSet, key)
		c.Queue.Forget(key)
	} else {
		c.Handler.ObjectCreated(c.ClientSet, item)
		c.Queue.Forget(key)
	}

	return true
}

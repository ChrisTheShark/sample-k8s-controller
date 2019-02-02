package main

import (
	"flag"
	"log"
	"os"

	"github.com/ChrisTheShark/base-k8s-controller/controller"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

type podNameLoggingHandler struct{}

func (s podNameLoggingHandler) ObjectCreated(client kubernetes.Interface, obj interface{}) {
	if pod, ok := obj.(*apiv1.Pod); ok {
		log.Printf("Pod created with name: %v\n", pod.GetName())
	}
}
func (s podNameLoggingHandler) ObjectDeleted(client kubernetes.Interface, obj interface{}) {
	if pod, ok := obj.(*apiv1.Pod); ok {
		log.Printf("Pod deleted with name: %v\n", pod.GetName())
	}
}

func main() {
	var kubeConf string
	if len(os.Args) > 1 {
		flag.StringVar(&kubeConf, "kubeConf", os.Args[1], "Path to a local .kube configuration")
	}
	client, err := getClient(kubeConf)
	if err != nil {
		log.Panicln("Error initialzing client:", err)
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return client.CoreV1().Pods(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.CoreV1().Pods(metav1.NamespaceAll).Watch(options)
			},
		},
		&apiv1.Pod{},
		0,
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	})

	c := controller.Controller{
		ClientSet: client,
		Queue:     queue,
		Informer:  informer,
		Handler:   podNameLoggingHandler{},
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	c.Run(stopCh)
}

// Allow connection via provided .kube/conf path, otherwise assume we are
// inside the cluster.
func getClient(kubeConf string) (kubernetes.Interface, error) {
	if kubeConf != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeConf)
		if err != nil {
			return nil, err
		}
		return kubernetes.NewForConfig(config)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

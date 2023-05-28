package watcher

import (
	"context"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Error struct {
	Resource schema.GroupVersionResource
	Error    error
}

type EventHandler interface {
	HandleAdd(obj interface{}) error
	HandleDelete(obj interface{}) error
	HandleUpdate(oldObj interface{}, newObj interface{}) error
}

type ResourceWatcher struct {
	Handler EventHandler
}

func NewFactory(clientSet kubernetes.Interface, resyncPeriod time.Duration) informers.SharedInformerFactory {
	return informers.NewSharedInformerFactory(clientSet, resyncPeriod)
}

func (rw *ResourceWatcher) NewDefaultGroupVersionResources() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
	}
}

func (rw *ResourceWatcher) NewEventHandlerFuncs() *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rw.Handler.HandleAdd(obj)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			rw.Handler.HandleUpdate(oldObj, newObj)
		},

		DeleteFunc: func(obj interface{}) {
			rw.Handler.HandleDelete(obj)
		},
	}
}

func (rw *ResourceWatcher) Watch(ctx context.Context, i informers.GenericInformer, h cache.ResourceEventHandler) error {
	_, err := i.Informer().AddEventHandler(h)
	if err != nil {
		log.Fatalf("could not attach event handlers to the informer: %v", err)
	}

	i.Informer().Run(ctx.Done())

	return nil
}

package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adykaaa/k8s-netpol-ctrl/config"
	"github.com/adykaaa/k8s-netpol-ctrl/handlers/attribute"
	"github.com/adykaaa/k8s-netpol-ctrl/handlers/event"
	"github.com/adykaaa/k8s-netpol-ctrl/handlers/networkpolicy"
	"github.com/adykaaa/k8s-netpol-ctrl/watcher"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type ResourceWatcher interface {
	Watch(ctx context.Context, i informers.GenericInformer, h cache.ResourceEventHandler) error
	NewEventHandlerFuncs() *cache.ResourceEventHandlerFuncs
}

type App struct {
	clientSet       kubernetes.Interface
	configProvider  config.Provider
	informerFactory informers.SharedInformerFactory
	gvrs            []schema.GroupVersionResource
	resourceWatcher ResourceWatcher
}

func New() (*App, error) {
	cp := &config.DefaultProvider{}
	config, err := config.New(cp)
	if err != nil {
		return nil, fmt.Errorf("could not initialize configuration: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not initialize clientSet: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not initialize dyamic client: %w", err)
	}

	rw := &watcher.ResourceWatcher{
		Handler: &event.Handler{
			Client:        clientSet,
			DyanmicClient: dynamicClient,
			NetworkPolicyHandler: &networkpolicy.Handler{
				Client: clientSet,
			},
			AttributeHandler: &attribute.Handler{
				Client: clientSet,
			},
		},
	}

	gvrs := rw.NewDefaultGroupVersionResources()
	informerFactory := watcher.NewFactory(clientSet, 30*time.Second)

	return &App{
		clientSet:       clientSet,
		configProvider:  cp,
		informerFactory: informerFactory,
		gvrs:            gvrs,
		resourceWatcher: rw,
	}, nil
}

func (a *App) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan watcher.Error, len(a.gvrs))
	ehf := a.resourceWatcher.NewEventHandlerFuncs()

	for _, gvr := range a.gvrs {
		go func(gvr schema.GroupVersionResource) {
			inf, err := a.informerFactory.ForResource(gvr)
			if err != nil {
				log.Fatalf("could not initialize informer for %v", gvr)
			}

			err = a.resourceWatcher.Watch(ctx, inf, ehf)
			if err != nil {
				log.Printf("could not start resource watcher: %v", err)
				errCh <- watcher.Error{Resource: gvr, Error: err}
				return
			}
		}(gvr)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			log.Println("signal received, stopping the application")
			cancel()
			return
		case we := <-errCh:
			log.Printf("resource watcher %v stopped with error: %v \n", we.Resource, we.Error)
		}
	}
}

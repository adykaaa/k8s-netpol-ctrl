package object

import (
	"context"
	"errors"
	"fmt"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Action uint8

const (
	Create Action = iota
	Update
	Delete
)

var (
	ErrTypeNotSupported = errors.New("this type is not supported")
	ErrNotFound         = errors.New("this resource does not exist")
)

type Handler struct {
	Client dynamic.Interface
	Obj    metav1.Object
}

func NewHandler(c dynamic.Interface, obj metav1.Object) *Handler {
	return &Handler{Client: c, Obj: obj}
}

/*
ConvertToMeta checks whether the object is of type Pod, Deployment, StatefulSet or DaemonSet, and converts it to metav1.Object,
and returns its labels
*/
func ConvertToMeta(obj interface{}) (map[string]string, metav1.Object, error) {
	if obj == nil {
		return nil, nil, errors.New("object cannot be nil")
	}

	switch obj := obj.(type) {
	case *corev1.Pod:
		for _, or := range obj.ObjectMeta.OwnerReferences {
			if or.Kind == "ReplicaSet" || or.Kind == "Deployment" || or.Kind == "StatefulSet" || or.Kind == "DaemonSet" {
				return nil, nil, fmt.Errorf("POD %s is part of a %s so skipping", obj.GetName(), or.Kind)
			}
		}
		return obj.Labels, obj, nil
	case *appsv1.Deployment:
		return obj.Spec.Selector.MatchLabels, obj, nil
	case *appsv1.StatefulSet:
		return obj.Spec.Selector.MatchLabels, obj, nil
	case *appsv1.DaemonSet:
		return obj.Spec.Selector.MatchLabels, obj, nil
	}
	return nil, nil, ErrTypeNotSupported
}

// AddLabel applies the "netpol-ctrl":"<obj_name>-<obj_namespace>" label to a metav1.Object if it does not yet have a label
func (h *Handler) AddLabel() error {
	label := map[string]string{"netpol-ctrl": fmt.Sprintf("%s-%s", h.Obj.GetName(), h.Obj.GetNamespace())}

	h.Obj.SetLabels(label)
	err := h.Mutate(Update)
	if err != nil {
		return fmt.Errorf("could not apply label to pod %s: %w", h.Obj.GetName(), err)
	}
	log.Printf("pod '%s' labeled! \n", h.Obj.GetName())
	return nil
}

/*
getResourceGVR takes in the MetaObject and returns the GVR of it, so that the dynamic client will know
on what object to call the Update() or Create() functions on
*/
func (h *Handler) getGVR() (schema.GroupVersionResource, error) {
	switch h.Obj.(type) {
	case *corev1.Pod:
		return corev1.SchemeGroupVersion.WithResource("pods"), nil
	case *appsv1.Deployment:
		return appsv1.SchemeGroupVersion.WithResource("deployments"), nil
	case *appsv1.StatefulSet:
		return appsv1.SchemeGroupVersion.WithResource("statefulsets"), nil
	case *appsv1.DaemonSet:
		return appsv1.SchemeGroupVersion.WithResource("daemonsets"), nil
	case *networkingv1.NetworkPolicy:
		return networkingv1.SchemeGroupVersion.WithResource("networkpolicies"), nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported object type: %T", h.Obj)
	}
}

// Mutate applies the modifications to a given metaObj based on the given Action
func (h *Handler) Mutate(action Action) error {
	objName := h.Obj.GetName()

	gvr, err := h.getGVR()
	if err != nil {
		return fmt.Errorf("error during resource gvr retrieval. %v", err)
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(h.Obj)
	if err != nil {
		return fmt.Errorf("error during object conversion to unstructured. %v", err)
	}
	resourceClient := h.Client.Resource(gvr).Namespace(h.Obj.GetNamespace())

	switch action {
	case Create:
		_, err = resourceClient.Create(context.Background(), &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create resource %v. %w", objName, err)
		}

	case Update:
		_, err = resourceClient.Update(context.Background(), &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("could not update resource %v. %w", objName, err)
		}

	case Delete:
		err = resourceClient.Delete(context.Background(), h.Obj.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("could not delete resource %v. %w", objName, err)
		}
	}

	return nil
}

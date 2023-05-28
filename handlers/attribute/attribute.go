// the attribute package deals with everything that is related to operations on K8s object labels and env. vars
package attribute

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrNoEnvVars        = errors.New("the object has no .cluster.local env vars")
	ErrEmptyParam       = errors.New("a required parameter is empty")
	ErrResourceNotFound = errors.New("this resource does not exist")
	ErrTypeNotSupported = errors.New("this type is not supported")
)

type Handler struct {
	Client kubernetes.Interface
}

// helper function to check if []T contains T
func Contains[T comparable](elems []T, v T) bool {
	if len(elems) == 0 {
		return false
	}

	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

// helper function to check if two maps are equal or not
func MapsEqual[K comparable, V comparable](a, b map[K]V) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if vb, ok := b[k]; !ok || v != vb {
			return false
		}
	}
	return true
}

// regex helper function to check whether the environment variable is a valid cluster.local env var -> <name>.<namespace>.svc/pod.cluster.local
func isValidLocalEnvVar(s string) bool {
	pattern := `^[a-zA-Z0-9-]+\.[a-zA-Z0-9-]+\.pod\.cluster\.local$|^[a-zA-Z0-9-]+\.[a-zA-Z0-9-]+\.svc\.cluster\.local$`
	matched, _ := regexp.MatchString(pattern, s)
	return matched
}

// appendUnique appends only the unique items from src to dst.
func appendUnique(dst, src []string) []string {
	exists := make(map[string]bool)
	for _, v := range dst {
		exists[v] = true
	}
	for _, v := range src {
		if !exists[v] {
			dst = append(dst, v)
			exists[v] = true
		}
	}
	return dst
}

// MergeLabels merges a variadic number of labels into one
func (h *Handler) MergeLabels(targetLabels ...map[string][]string) (map[string][]string, error) {
	if len(targetLabels) == 0 {
		return nil, ErrEmptyParam
	}

	mergedLabels := make(map[string][]string)

	for _, target := range targetLabels {
		for k, v := range target {
			mergedLabels[k] = appendUnique(mergedLabels[k], v)
		}
	}

	return mergedLabels, nil
}

// ConvertMap converts K8s object labels (map[string]string) to map[string][]string. e.g {"label":"value"} -> label: {"value"}
func (h *Handler) ConvertLabels(targetPodLabels map[string]string) map[string][]string {
	newMap := make(map[string][]string)
	for k, v := range targetPodLabels {
		newMap[k] = []string{v}
	}
	return newMap
}

// GetLocalEnvVars fetches all env. vars with the values of <name>.<namespace>.svc/pod.cluster.local
func (h *Handler) GetLocalEnvVars(obj metav1.Object) (map[string]string, error) {
	var containers []corev1.Container
	envVars := make(map[string]string)

	switch obj := obj.(type) {
	case *corev1.Pod:
		containers = obj.Spec.Containers
	case *appsv1.Deployment:
		containers = obj.Spec.Template.Spec.Containers
	case *appsv1.StatefulSet:
		containers = obj.Spec.Template.Spec.Containers
	case *appsv1.DaemonSet:
		containers = obj.Spec.Template.Spec.Containers
	default:
		return nil, ErrTypeNotSupported
	}

	for _, container := range containers {
		for _, envVar := range container.Env {
			if (strings.Contains(envVar.Value, ".svc.cluster.local") || strings.Contains(envVar.Value, ".pod.cluster.local")) && isValidLocalEnvVar(envVar.Value) {
				envVars[envVar.Name] = envVar.Value
			}
		}
	}

	return envVars, nil
}

/*
GetLabelsFromEnvVars takes in the environment variables containing "<name>.<namespace>.svc.cluster.local" and "<name>.<namespace>.pod.cluster.local". Then for PODs, it returns
all the labels it has and for services it looks at the .spec.Selector (which are esentially the POD labels it targets) and returns those.
It returns a map[string][]string because it can happen that two pods have the same label keys with different values.

	e.g	{
		    "app": ["test", "frontend", "backend"],
		    "version": ["1.0", "2.0"],
		    "env": ["prod"],
		}
*/
func (h *Handler) GetLabelsFromEnvVars(envVars map[string]string) (map[string][]string, error) {
	labels := make(map[string][]string)

	if len(envVars) == 0 {
		return nil, ErrNoEnvVars
	}

	for _, v := range envVars {
		name := strings.Split(v, ".")[0]
		namespace := strings.Split(v, ".")[1]

		switch strings.Contains(v, ".pod.cluster.local") {
		case true:
			pod, err := h.Client.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return nil, ErrResourceNotFound
				}
				return nil, fmt.Errorf("could not fetch pod %s. %w", name, err)
			}
			for k, v := range pod.ObjectMeta.Labels {
				labels[k] = append(labels[k], v)
			}
		default:
			svcLabels, err := h.getLabelsFromSvc(name, namespace)
			if err != nil {
				return nil, err
			}
			for k, v := range svcLabels {
				labels[k] = append(labels[k], v)
			}
		}
	}

	return labels, nil
}

// GetLabelsFromSvc returns all the POD selectors a Service has
func (h *Handler) getLabelsFromSvc(name string, namespace string) (map[string]string, error) {
	labels := map[string]string{}
	svc, err := h.Client.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, ErrResourceNotFound
		}
		return nil, fmt.Errorf("could not fetch svc %s. %w", name, err)
	}
	for k, v := range svc.Spec.Selector {
		labels[k] = v
	}

	if len(labels) == 0 {
		return nil, ErrResourceNotFound
	}

	return labels, nil
}

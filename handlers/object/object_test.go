package object

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stest "k8s.io/client-go/testing"
)

// helper function to deploy a Pod using the handler's fake dynamic client and returns the GroupVersionResource Schema for it
func setupTestPod(t *testing.T, h *Handler) schema.GroupVersionResource {
	t.Helper()
	gvr, err := h.getGVR()
	if err != nil {
		t.Fatalf("could not get gvr. %v", err)
	}
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(h.Obj)
	if err != nil {
		t.Fatalf("error during unstructured conversion in test. %v", err)
	}

	_, err = h.Client.Resource(gvr).Namespace(h.Obj.GetNamespace()).Create(context.Background(), &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create test pod %v", err)
	}

	return gvr
}

func TestConvertToMetaObj(t *testing.T) {
	testCases := []struct {
		name    string
		input   interface{}
		testErr func(t *testing.T, err error)
	}{
		{
			name:  "empty input obj",
			input: nil,
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:  "unsupported type",
			input: "unsupported type",
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:  "standalone Pod",
			input: &corev1.Pod{},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Pod owned by Deployment",
			input: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Deployment",
						},
					},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "Deployment",
			input: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "StatefulSet",
			input: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "DaemonSet",
			input: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ConvertToMeta(tc.input)
			tc.testErr(t, err)
		})
	}
}

func TestAddLabel(t *testing.T) {
	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	h := Handler{Obj: obj, Client: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())}
	s := setupTestPod(t, &h)

	err := h.AddLabel()
	if err != nil {
		t.Errorf("AddLabel() error = %v", err)
		return
	}

	result, getErr := h.Client.Resource(s).Namespace(obj.Namespace).Get(context.Background(), obj.GetName(), metav1.GetOptions{})
	if getErr != nil {
		t.Errorf("Error fetching updated object: %v", getErr)
		return
	}

	label := result.GetLabels()
	expectedLabelValue := fmt.Sprintf("%s-%s", obj.GetName(), obj.GetNamespace())

	if label["netpol-ctrl"] != expectedLabelValue {
		t.Errorf("label does not match expected: got %s want %s", label["netpol-ctrl"], expectedLabelValue)
	}
}

func TestGetResourceGVR(t *testing.T) {
	handlerPod := &Handler{Obj: &corev1.Pod{}}
	handlerDeployment := &Handler{Obj: &appsv1.Deployment{}}
	handlerIngress := &Handler{Obj: &networkingv1.Ingress{}}
	handlerStatefulSet := &Handler{Obj: &appsv1.StatefulSet{}}
	handlerDaemonSet := &Handler{Obj: &appsv1.DaemonSet{}}
	handlerNetworkPolicy := &Handler{Obj: &networkingv1.NetworkPolicy{}}

	testCases := []struct {
		name    string
		handler *Handler
		want    schema.GroupVersionResource
		testErr func(t *testing.T, err error)
	}{
		{
			name:    "OK - Pod",
			handler: handlerPod,
			want:    corev1.SchemeGroupVersion.WithResource("pods"),
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:    "OK - Deployment",
			handler: handlerDeployment,
			want:    appsv1.SchemeGroupVersion.WithResource("deployments"),
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:    "OK - DaemonSets",
			handler: handlerDaemonSet,
			want:    appsv1.SchemeGroupVersion.WithResource("daemonsets"),
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:    "OK - StatefulSet",
			handler: handlerStatefulSet,
			want:    appsv1.SchemeGroupVersion.WithResource("statefulsets"),
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:    "OK - NetworkPolicy",
			handler: handlerNetworkPolicy,
			want:    networkingv1.SchemeGroupVersion.WithResource("networkpolicies"),
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:    "fails - not supported type",
			handler: handlerIngress,
			want:    schema.GroupVersionResource{},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.handler.getGVR()
			tc.testErr(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMutate(t *testing.T) {
	testCases := []struct {
		name       string
		action     Action
		wantK8sErr bool
		obj        metav1.Object
		testErr    func(t *testing.T, err error)
	}{
		{
			name:       "OK - create",
			action:     Create,
			obj:        &corev1.Pod{},
			wantK8sErr: false,
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:       "OK - update",
			action:     Update,
			obj:        &corev1.Pod{},
			wantK8sErr: false,
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:       "OK - delete",
			action:     Delete,
			obj:        &corev1.Pod{},
			wantK8sErr: false,
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:       "fails - not supported gvr",
			action:     Create,
			obj:        &networkingv1.Ingress{},
			wantK8sErr: false,
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:       "fails - internal error while creating object",
			action:     Create,
			obj:        &corev1.Pod{},
			wantK8sErr: true,
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:       "fails - internal error while updating object",
			action:     Update,
			obj:        &corev1.Pod{},
			wantK8sErr: true,
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:       "fails - internal error while deleting object",
			action:     Delete,
			obj:        &corev1.Pod{},
			wantK8sErr: true,
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := Handler{Obj: tc.obj, Client: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())}
			switch tc.action {
			case Update:
				_ = setupTestPod(t, &h)
			case Delete:
				_ = setupTestPod(t, &h)
			}

			if tc.wantK8sErr {
				h.Client.(*dynamicfake.FakeDynamicClient).PrependReactor("*", "*", func(action k8stest.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("forced %s error", action.GetVerb()))
				})
			}

			err := h.Mutate(tc.action)
			tc.testErr(t, err)
		})
	}
}

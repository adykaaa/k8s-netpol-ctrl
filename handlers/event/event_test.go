package event

import (
	"context"
	"reflect"
	"testing"

	"github.com/adykaaa/k8s-netpol-ctrl/handlers/attribute"
	"github.com/adykaaa/k8s-netpol-ctrl/handlers/networkpolicy"
	"github.com/adykaaa/k8s-netpol-ctrl/handlers/object"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func deployPolicyForDynamic(t *testing.T, client dynamic.Interface) (*networkingv1.NetworkPolicy, error) {
	t.Helper()

	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpolicy",
			Namespace: "testnamespace",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"test"},
					},
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"test"},
									},
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"test"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(networkPolicy)
	if err != nil {
		t.Fatalf("failed to convert NetworkPolicy to unstructured: %v", err)
	}

	p := &unstructured.Unstructured{Object: unstructuredObj}

	gvr := schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "networkpolicies",
	}

	unstructuredPolicy, err := client.Resource(gvr).Namespace(networkPolicy.Namespace).Create(context.Background(), p, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test policy")
	}
	createdNetworkPolicy := &networkingv1.NetworkPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPolicy.UnstructuredContent(), createdNetworkPolicy); err != nil {
		t.Fatalf("couldnt convert Unstructured to NetworkPolicy")
	}

	return createdNetworkPolicy, nil

}

func getAllNetworkPolicies(t *testing.T, client dynamic.Interface) ([]networkingv1.NetworkPolicy, error) {
	t.Helper()

	var networkPolicies []networkingv1.NetworkPolicy
	gvr := schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "networkpolicies",
	}
	list, err := client.Resource(gvr).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		var networkPolicy networkingv1.NetworkPolicy
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &networkPolicy); err != nil {
			return nil, err
		}
		networkPolicies = append(networkPolicies, networkPolicy)
	}

	return networkPolicies, nil
}

func returnTestPod(t *testing.T, labels map[string]string) *corev1.Pod {
	t.Helper()

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testname",
			Namespace: "testnamespace",
			Labels:    labels,
		},
	}
}

func returnTestSvc(t *testing.T, labels map[string]string) *corev1.Service {
	t.Helper()

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testname",
			Namespace: "testnamespace",
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
		},
	}
}

func deployPolicyForSimple(t *testing.T, c kubernetes.Interface, podSelector map[string]string) (*networkingv1.NetworkPolicy, error) {
	t.Helper()

	p := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpolicy",
			Namespace: "testnamespace",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: podSelector,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"test"},
									},
									{
										Key:      "label2",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"value2"},
									},
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"test"},
									},
									{
										Key:      "label2",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"value2"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	p, err := c.NetworkingV1().NetworkPolicies("testnamespace").Create(context.Background(), p, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("error during test networkpolicy deployment %v", err)
	}
	return p, nil
}

func containsLabelSelectorReq(t *testing.T, targets []metav1.LabelSelectorRequirement, policy *networkingv1.NetworkPolicy) bool {
	t.Helper()

	for _, target := range targets {
		isInIngress := false
		isInEgress := false

		for _, ingressRule := range policy.Spec.Ingress {
			for _, peer := range ingressRule.From {
				if peer.PodSelector != nil {
					for _, req := range peer.PodSelector.MatchExpressions {
						if reflect.DeepEqual(target, req) {
							isInIngress = true
							break
						}
					}
				}
			}
		}

		for _, egressRule := range policy.Spec.Egress {
			for _, peer := range egressRule.To {
				if peer.PodSelector != nil {
					for _, req := range peer.PodSelector.MatchExpressions {
						if reflect.DeepEqual(target, req) {
							isInEgress = true
							break
						}
					}
				}
			}
		}

		if !(isInEgress && isInIngress) {
			return false
		}
	}

	return true
}

func TestHandleLabelChange(t *testing.T) {
	testCases := []struct {
		name                     string
		newLabels                map[string]string
		expectedLabelSelectorReq []metav1.LabelSelectorRequirement
		testErr                  func(t *testing.T, err error)
	}{
		{
			name:      "OK - old reqs removed, new one added",
			newLabels: map[string]string{"label1": "value1"},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "label1",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value1"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:      "OK - labels removed",
			newLabels: map[string]string{"app": "test"},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:      "errors - new labels empty",
			newLabels: map[string]string{},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewSimpleClientset()
			h := &Handler{
				Client: c,
				AttributeHandler: &attribute.Handler{
					Client: c,
				},
				NetworkPolicyHandler: &networkpolicy.Handler{
					Client: c,
				},
			}
			oldLabels := map[string]string{"app": "test", "label2": "value2"}
			p, _ := deployPolicyForSimple(t, h.Client, oldLabels)

			err := h.handleLabelChange(oldLabels, tc.newLabels, p)
			tc.testErr(t, err)

			for _, er := range p.Spec.Egress {
				for _, peer := range er.To {
					reflect.DeepEqual(peer.PodSelector.MatchExpressions, tc.expectedLabelSelectorReq)
				}
			}

			for _, ir := range p.Spec.Ingress {
				for _, peer := range ir.From {
					reflect.DeepEqual(peer.PodSelector.MatchExpressions, tc.expectedLabelSelectorReq)
				}
			}

		})
	}
}

func TestHandleEnvVarChange(t *testing.T) {
	testCases := []struct {
		name                     string
		inputEnvVars             map[string]string
		inputObjLabels           map[string]string
		testObj                  string
		expectedLabelSelectorReq []metav1.LabelSelectorRequirement
		testErr                  func(t *testing.T, err error)
	}{
		{
			name:           "OK - adding Pod labels to NetworkPolicy",
			inputEnvVars:   map[string]string{"test": "testname.testnamespace.pod.cluster.local"},
			inputObjLabels: map[string]string{"podlabel": "value1"},
			testObj:        "pod",
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "podlabel",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value1"},
				},
			},

			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:           "OK - adding Svc -> Pod labels to NetworkPolicy",
			inputEnvVars:   map[string]string{"test": "testname.testnamespace.svc.cluster.local"},
			inputObjLabels: map[string]string{"svclabel": "value1"},
			testObj:        "svc",
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "svclabel",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value1"},
				},
			},

			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:           "fails - object doesn't exist in the cluster",
			inputEnvVars:   map[string]string{"test": "nonexistent.stuff.svc.cluster.local"},
			inputObjLabels: map[string]string{"svclabel": "value1"},
			testObj:        "svc",
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "randomlabel",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value1"},
				},
			},

			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:           "fails - no env vars",
			inputEnvVars:   map[string]string{},
			inputObjLabels: map[string]string{"svclabel": "value1"},
			testObj:        "svc",
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{},
				},
			},

			testErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, attribute.ErrNoEnvVars)
			},
		},
		{
			name:           "fails - no input labels",
			inputEnvVars:   map[string]string{"test": "testname.testnamespace.svc.cluster.local"},
			inputObjLabels: map[string]string{},
			testObj:        "svc",
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{},
				},
			},

			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var envErr error
			c := fake.NewSimpleClientset()

			h := &Handler{
				Client: c,
				AttributeHandler: &attribute.Handler{
					Client: c,
				},
				NetworkPolicyHandler: &networkpolicy.Handler{
					Client: c,
				},
			}

			policy, _ := deployPolicyForSimple(t, h.Client, tc.inputObjLabels)

			switch tc.testObj {
			case "pod":
				pod := returnTestPod(t, tc.inputObjLabels)

				p, err := h.Client.CoreV1().Pods(pod.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("test pod creation failed. %v", err)
				}

				envErr = h.handleEnvVarChange(p.GetLabels(), tc.inputEnvVars, policy)
				tc.testErr(t, envErr)

			case "svc":
				svc := returnTestSvc(t, tc.inputObjLabels)

				s, err := h.Client.CoreV1().Services(svc.GetNamespace()).Create(context.Background(), svc, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("test pod creation failed. %v", err)
				}
				envErr = h.handleEnvVarChange(s.Spec.Selector, tc.inputEnvVars, policy)
				tc.testErr(t, envErr)
			}

			_, err := h.Client.NetworkingV1().NetworkPolicies(policy.Namespace).Update(context.Background(), policy, metav1.UpdateOptions{})
			if err != nil {
				t.Fatalf("test policy update failed! %v", err)
			}

			if envErr == nil && !containsLabelSelectorReq(t, tc.expectedLabelSelectorReq, policy) {
				t.Errorf("policy not modified as expected. Expected: %v, Actual %v", tc.expectedLabelSelectorReq, policy.Spec.Ingress[0].From)
			}

		})
	}
}

func TestHandleAdd(t *testing.T) {
	testCases := []struct {
		name                     string
		obj                      interface{}
		expectedLabelSelectorReq []metav1.LabelSelectorRequirement
		envObjDeploy             func(t *testing.T, c kubernetes.Interface)
		testErr                  func(t *testing.T, err error)
	}{
		{
			name: "OK - NetworkPolicy created for POD without envvars",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testname",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			envObjDeploy: func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "OK - NetworkPolicy created for POD (envvars are not relevant) ",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testname",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "containername",
							Image: "containerimage",
							Env: []corev1.EnvVar{
								{
									Name:  "NONEXISTENT",
									Value: "non.existent.pod.cluster.local",
								},
							},
						},
					},
				},
			},
			envObjDeploy: func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "OK - NetworkPolicy created for POD + POD env.var labels included",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testname",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "containername",
							Image: "containerimage",
							Env: []corev1.EnvVar{
								{
									Name:  "OK",
									Value: "testpod.testnamespace.pod.cluster.local",
								},
							},
						},
					},
				},
			},
			envObjDeploy: func(t *testing.T, c kubernetes.Interface) {
				p := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testpod",
						Namespace: "testnamespace",
						Labels:    map[string]string{"env": "ok"},
					},
				}
				_, err := c.CoreV1().Pods(p.Namespace).Create(context.Background(), p, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("error creating test POD %v", err)
				}
			},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
				{
					Key:      "env",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"ok"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:                     "error - cannot convert to relevant MetaObj",
			obj:                      "bad_obj",
			envObjDeploy:             func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "error - kube-system ns",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "kube-system",
				},
			},
			envObjDeploy:             func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewSimpleClientset()
			dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
				{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}: "NetworkPolicyList",
				{Group: "", Version: "v1", Resource: "pods"}:                             "PodList",
				{Group: "", Version: "v1", Resource: "services"}:                         "ServiceList",
			})

			h := &Handler{
				Client:        c,
				DyanmicClient: dc,
				NetworkPolicyHandler: &networkpolicy.Handler{
					Client: c,
				},
				AttributeHandler: &attribute.Handler{
					Client: c,
				},
			}

			tc.envObjDeploy(t, h.Client)
			err := h.HandleAdd(tc.obj)
			tc.testErr(t, err)

			if err != nil {
				allPolicies, err := getAllNetworkPolicies(t, h.DyanmicClient)
				if err != nil {
					t.Fatalf("error during retrieving all test policies")
				}

				for _, p := range allPolicies {
					if !containsLabelSelectorReq(t, tc.expectedLabelSelectorReq, &p) {
						t.Errorf("%v \n", p)
						t.Fatalf("policy does not contain LabelSelectorReq %v", tc.expectedLabelSelectorReq)
					}

				}
			}
		})
	}
}

func TestHandleUpdate(t *testing.T) {
	testCases := []struct {
		name                     string
		oldObj                   interface{}
		newObj                   interface{}
		deployAuxObj             func(t *testing.T, c kubernetes.Interface)
		expectedLabelSelectorReq []metav1.LabelSelectorRequirement
		shouldNotContainReq      []metav1.LabelSelectorRequirement
		testErr                  func(t *testing.T, err error)
	}{
		{
			name: "OK - labels or env vars haven't changed, returns nil",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			deployAuxObj:             func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{},
			shouldNotContainReq:      []metav1.LabelSelectorRequirement{},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "OK - labels changed",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "changed"},
				},
			},
			deployAuxObj: func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"changed"},
				},
			},
			shouldNotContainReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "OK - labels changed - removed old, added 2 new",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"label1": "value1", "label2": "value2"},
				},
			},
			deployAuxObj: func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "label1",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value1"},
				},
				{
					Key:      "label2",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value2"},
				},
			},
			shouldNotContainReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "OK - envvars changed, aux. obj labels added to policy",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "containername",
							Image: "containerimage",
							Env: []corev1.EnvVar{
								{
									Name:  "OLD",
									Value: "old.envvar.pod.cluster.local",
								},
							},
						},
					},
				},
			},
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"label1": "value1", "label2": "value2"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "containername",
							Image: "containerimage",
							Env: []corev1.EnvVar{
								{
									Name:  "POD",
									Value: "test.default.pod.cluster.local",
								},
							},
						},
					},
				},
			},
			deployAuxObj: func(t *testing.T, c kubernetes.Interface) {
				obj := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
						Labels:    map[string]string{"pod": "fromenv"},
					},
				}
				_, err := c.CoreV1().Pods(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("could not deploy test auxillary object")
				}
			},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "label1",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value1"},
				},
				{
					Key:      "label2",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"value2"},
				},
				{
					Key:      "pod",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"fromenv"},
				},
			},
			shouldNotContainReq: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "errors - invalid newobj",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			newObj:                   "invalid",
			deployAuxObj:             func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{},
			shouldNotContainReq:      []metav1.LabelSelectorRequirement{},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:   "erros - invalid oldobj",
			oldObj: "invalid",
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			deployAuxObj:             func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{},
			shouldNotContainReq:      []metav1.LabelSelectorRequirement{},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "errors - kube-system namespace",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "kube-system",
					Labels:    map[string]string{"app": "test"},
				},
			},
			deployAuxObj:             func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{},
			shouldNotContainReq:      []metav1.LabelSelectorRequirement{},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "errors - no labels on new obj",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "kube-system",
				},
			},
			deployAuxObj:             func(t *testing.T, c kubernetes.Interface) {},
			expectedLabelSelectorReq: []metav1.LabelSelectorRequirement{},
			shouldNotContainReq:      []metav1.LabelSelectorRequirement{},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewSimpleClientset()
			dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
				{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}: "NetworkPolicyList",
				{Group: "", Version: "v1", Resource: "pods"}:                             "PodList",
				{Group: "", Version: "v1", Resource: "services"}:                         "ServiceList",
			})

			h := &Handler{
				Client:        c,
				DyanmicClient: dc,
				NetworkPolicyHandler: &networkpolicy.Handler{
					Client: c,
				},
				AttributeHandler: &attribute.Handler{
					Client: c,
				},
				ObjectHandler: &object.Handler{
					Client: dc,
				},
			}

			tc.deployAuxObj(t, h.Client)
			err := h.HandleUpdate(tc.oldObj, tc.newObj)
			tc.testErr(t, err)

			if err == nil {
				allPolicies, err := getAllNetworkPolicies(t, h.DyanmicClient)
				if err != nil {
					t.Fatalf("error during retrieving all test policies")
				}
				if len(allPolicies) > 1 {
					t.Fatalf("there should only be 1 policy in the test")
				}

				for _, p := range allPolicies {
					if !containsLabelSelectorReq(t, tc.expectedLabelSelectorReq, &p) {
						t.Fatalf("policy does not contain LabelSelectorReq %v", tc.expectedLabelSelectorReq)
					}

					if len(tc.shouldNotContainReq) > 0 && containsLabelSelectorReq(t, tc.shouldNotContainReq, &p) {
						t.Fatalf("policy %v should not contain %v", p, tc.shouldNotContainReq)
					}

				}
			}

		})
	}
}

func TestHandleDelete(t *testing.T) {
	testCases := []struct {
		name    string
		obj     interface{}
		testErr func(t *testing.T, err error)
	}{
		{
			name: "OK",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"app": "test"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "errors - returns policy not found",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{"non": "existing"},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "errors - kube-system namespace",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "kube-system",
					Labels:    map[string]string{},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "errors - no labels",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "testnamespace",
					Labels:    map[string]string{},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewSimpleClientset()
			dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
				{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}: "NetworkPolicyList",
				{Group: "", Version: "v1", Resource: "pods"}:                             "PodList",
				{Group: "", Version: "v1", Resource: "services"}:                         "ServiceList",
			})

			h := &Handler{
				Client:        c,
				DyanmicClient: dc,
				NetworkPolicyHandler: &networkpolicy.Handler{
					Client: c,
				},
				ObjectHandler: &object.Handler{
					Client: dc,
				},
			}
			//create policy for simple and dynamic client as well
			_, _ = deployPolicyForDynamic(t, h.DyanmicClient)
			_, _ = deployPolicyForSimple(t, h.Client, tc.obj.(*corev1.Pod).GetLabels())

			err := h.HandleDelete(tc.obj)
			tc.testErr(t, err)

		})
	}
}

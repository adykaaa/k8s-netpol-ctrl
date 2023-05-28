package networkpolicy

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/adykaaa/k8s-netpol-ctrl/handlers/attribute"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func containsSameLabelSelectorReq(t *testing.T, peers1, peers2 []v1.NetworkPolicyPeer) bool {
	t.Helper()

	if len(peers1) != len(peers2) {
		return false
	}

	// Convert slices to maps for comparison
	map1 := make(map[string]*v1.NetworkPolicyPeer)
	for i := range peers1 {
		peer := &peers1[i]
		for _, expr := range peer.PodSelector.MatchExpressions {
			key := fmt.Sprintf("%s:%s", expr.Key, strings.Join(expr.Values, ","))
			map1[key] = peer
		}
	}

	map2 := make(map[string]*v1.NetworkPolicyPeer)
	for i := range peers2 {
		peer := &peers2[i]
		for _, expr := range peer.PodSelector.MatchExpressions {
			key := fmt.Sprintf("%s:%s", expr.Key, strings.Join(expr.Values, ","))
			map2[key] = peer
		}
	}

	// Compare maps
	return reflect.DeepEqual(map1, map2)
}

// helper function to check whether any of the NetworkPolicyPeers contain the targetPodLabels and namespaceMatchLabels
func checkSelectors(t *testing.T, peers []networkingv1.NetworkPolicyPeer, targetPodLabels map[string][]string) map[string]map[string]bool {
	t.Helper()
	containsPodSelector := make(map[string]map[string]bool)

	for _, peer := range peers {
		if peer.PodSelector != nil {
			for _, req := range peer.PodSelector.MatchExpressions {
				if values, ok := targetPodLabels[req.Key]; ok {
					if containsPodSelector[req.Key] == nil {
						containsPodSelector[req.Key] = make(map[string]bool)
					}
					for _, v := range req.Values {
						for _, targetValue := range values {
							if targetValue == v {
								containsPodSelector[req.Key][v] = true
								break
							}
						}
					}
				}
			}
		}
	}

	return containsPodSelector
}

func TestIsInOldLabels(t *testing.T) {
	tests := []struct {
		name      string
		req       metav1.LabelSelectorRequirement
		oldLabels map[string][]string
		want      bool
	}{
		{
			name: "label in old labels",
			req: metav1.LabelSelectorRequirement{
				Key:    "app",
				Values: []string{"web"},
			},
			oldLabels: map[string][]string{
				"app": {"web", "db"},
			},
			want: true,
		},
		{
			name: "label not in old labels",
			req: metav1.LabelSelectorRequirement{
				Key:    "app",
				Values: []string{"api"},
			},
			oldLabels: map[string][]string{
				"app": {"web", "db"},
			},
			want: false,
		},
		{
			name: "old labels are empty",
			req: metav1.LabelSelectorRequirement{
				Key:    "app",
				Values: []string{"web"},
			},
			oldLabels: map[string][]string{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInOldLabels(tt.req, tt.oldLabels); got != tt.want {
				t.Errorf("isInOldLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateIngressRules(t *testing.T) {
	tests := []struct {
		name        string
		oldLabels   map[string][]string
		ingressRule networkingv1.NetworkPolicyIngressRule
		want        []metav1.LabelSelectorRequirement
	}{
		{
			name: "OK - removes matching labels",
			oldLabels: map[string][]string{
				"app": {"web"},
			},
			ingressRule: networkingv1.NetworkPolicyIngressRule{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:    "app",
									Values: []string{"web"},
								},
								{
									Key:    "tier",
									Values: []string{"frontend"},
								},
							},
						},
					},
				},
			},
			want: []metav1.LabelSelectorRequirement{
				{
					Key:    "tier",
					Values: []string{"frontend"},
				},
			},
		},
		{
			name:      "no old labels",
			oldLabels: map[string][]string{},
			ingressRule: networkingv1.NetworkPolicyIngressRule{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:    "app",
									Values: []string{"web"},
								},
								{
									Key:    "tier",
									Values: []string{"frontend"},
								},
							},
						},
					},
				},
			},
			want: []metav1.LabelSelectorRequirement{
				{
					Key:    "app",
					Values: []string{"web"},
				},
				{
					Key:    "tier",
					Values: []string{"frontend"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingressRules := []networkingv1.NetworkPolicyIngressRule{tt.ingressRule}
			updateIngressRules(tt.oldLabels, ingressRules)
			if !reflect.DeepEqual(ingressRules[0].From[0].PodSelector.MatchExpressions, tt.want) {
				t.Errorf("updateIngressRules() = %v, want %v", ingressRules[0].From[0].PodSelector.MatchExpressions, tt.want)
			}
		})
	}
}

func TestUpdateEgressRules(t *testing.T) {
	tests := []struct {
		name       string
		oldLabels  map[string][]string
		egressRule networkingv1.NetworkPolicyEgressRule
		want       []metav1.LabelSelectorRequirement
	}{
		{
			name: "remove matching labels",
			oldLabels: map[string][]string{
				"app": {"web"},
			},
			egressRule: networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:    "app",
									Values: []string{"web"},
								},
								{
									Key:    "tier",
									Values: []string{"frontend"},
								},
							},
						},
					},
				},
			},
			want: []metav1.LabelSelectorRequirement{
				{
					Key:    "tier",
					Values: []string{"frontend"},
				},
			},
		},
		{
			name:      "no old labels",
			oldLabels: map[string][]string{},
			egressRule: networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:    "app",
									Values: []string{"web"},
								},
								{
									Key:    "tier",
									Values: []string{"frontend"},
								},
							},
						},
					},
				},
			},
			want: []metav1.LabelSelectorRequirement{
				{
					Key:    "app",
					Values: []string{"web"},
				},
				{
					Key:    "tier",
					Values: []string{"frontend"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			egressRules := []networkingv1.NetworkPolicyEgressRule{tt.egressRule}
			updateEgressRules(tt.oldLabels, egressRules)
			if !reflect.DeepEqual(egressRules[0].To[0].PodSelector.MatchExpressions, tt.want) {
				t.Errorf("updateEgressRules() = %v, want %v", egressRules[0].To[0].PodSelector.MatchExpressions, tt.want)
			}
		})
	}
}

func TestGetDefaultSupportedPeers(t *testing.T) {
	tests := []struct {
		name           string
		defaultLabels  map[string]map[string]string
		expectedOutput []networkingv1.NetworkPolicyPeer
	}{
		{
			name:           "Empty default labels",
			defaultLabels:  map[string]map[string]string{},
			expectedOutput: []networkingv1.NetworkPolicyPeer{},
		},
		{
			name: "Single key-value pair",
			defaultLabels: map[string]map[string]string{
				"default": {
					"app": "test",
				},
			},
			expectedOutput: []networkingv1.NetworkPolicyPeer{
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
		{
			name: "Multiple key-value pairs with different keys",
			defaultLabels: map[string]map[string]string{
				"default1": {
					"app": "test",
				},
				"default2": {
					"env": "prod",
				},
			},
			expectedOutput: []networkingv1.NetworkPolicyPeer{
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
				{
					PodSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "env",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"prod"},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := getDefaultSupportedPeers(test.defaultLabels)
			if !containsSameLabelSelectorReq(t, output, test.expectedOutput) {
				t.Errorf("Got output %v, expected %v", output, test.expectedOutput)
			}
		})
	}
}

func TestAppendLabelsToPeers(t *testing.T) {
	testCases := []struct {
		name            string
		targetPodLabels map[string][]string
		testErr         func(t *testing.T, err error)
	}{
		{
			name:            "OK",
			targetPodLabels: map[string][]string{"app": {"test"}},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:            "returns ErrEmptyParam",
			targetPodLabels: map[string][]string{},
			testErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrEmptyParam)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{}
			ingressPeers, egressPeers, err := h.AppendLabelsToPeers(tc.targetPodLabels)
			tc.testErr(t, err)
			if err == nil {
				containsPodSelector := checkSelectors(t, ingressPeers, tc.targetPodLabels)
				for k, values := range tc.targetPodLabels {
					for _, v := range values {
						if !containsPodSelector[k][v] {
							t.Errorf("expected PodSelector in ingressPeers to contain label %s with value %s", k, v)
						}
					}
				}

				containsPodSelector = checkSelectors(t, egressPeers, tc.targetPodLabels)
				for k, values := range tc.targetPodLabels {
					for _, v := range values {
						if !containsPodSelector[k][v] {
							t.Errorf("expected PodSelector in egressPeers to contain label %s with value %s", k, v)
						}
					}
				}
			}
		})
	}
}

func TestNewPolicy(t *testing.T) {
	testCases := []struct {
		name              string
		policyName        string
		namespace         string
		podSelectorLabels map[string]string
		targetPodLabels   map[string][]string
		testErr           func(t *testing.T, err error) bool
	}{
		{
			name:              "OK",
			policyName:        "testpolicy",
			namespace:         "default",
			podSelectorLabels: map[string]string{"app": "test"},
			targetPodLabels:   map[string][]string{"env": {"test"}},
			testErr: func(t *testing.T, err error) bool {
				assert.NoError(t, err)
				return false
			},
		},
		{
			name:              "returns ErrEmptyParam",
			policyName:        "testpolicy",
			namespace:         "",
			podSelectorLabels: map[string]string{"app": "test"},
			targetPodLabels:   map[string][]string{"env": {"test"}},
			testErr: func(t *testing.T, err error) bool {
				assert.ErrorIs(t, err, ErrEmptyParam)
				return true
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{
				Client: fake.NewSimpleClientset(),
			}
			p, err := h.NewPolicy(tc.policyName, tc.namespace, tc.podSelectorLabels, tc.targetPodLabels)
			isErr := tc.testErr(t, err)

			if !isErr {
				policy, err := h.Client.NetworkingV1().NetworkPolicies(tc.namespace).Create(context.Background(), p, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("expected no error getting NetworkPolicy, got %v", err)
				}

				if policy.ObjectMeta.Name != tc.policyName {
					t.Errorf("expected name %s, got %s", tc.name, policy.ObjectMeta.Name)
				}
				if policy.ObjectMeta.Namespace != tc.namespace {
					t.Errorf("expected namespace %s, got %s", tc.namespace, policy.ObjectMeta.Namespace)
				}
				if !attribute.MapsEqual(policy.Spec.PodSelector.MatchLabels, tc.podSelectorLabels) {
					t.Errorf("Expected %s, got %s", tc.podSelectorLabels, policy.Spec.PodSelector.MatchLabels)
				}

				for _, ingressRule := range policy.Spec.Ingress {
					containsPodSelector := checkSelectors(t, ingressRule.From, tc.targetPodLabels)
					for k, v := range tc.targetPodLabels {
						for _, val := range v {
							if !containsPodSelector[k][val] {
								t.Errorf("expected PodSelector to contain label %v with value %v", k, val)
							}
						}
					}
				}

				for _, egressRule := range policy.Spec.Egress {
					containsPodSelector := checkSelectors(t, egressRule.To, tc.targetPodLabels)
					for k, v := range tc.targetPodLabels {
						for _, val := range v {
							if !containsPodSelector[k][val] {
								t.Errorf("expected PodSelector to contain label %v with value %v", k, val)
							}
						}
					}
				}
			}
		})
	}
}

func TestGetPolicyByPodLabels(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
		podLabels map[string]string
		policies  []*networkingv1.NetworkPolicy
		expected  *networkingv1.NetworkPolicy
		testErr   func(t *testing.T, err error)
	}{
		{
			name:      "OK",
			namespace: "default",
			podLabels: map[string]string{"app": "test"},
			policies: []*networkingv1.NetworkPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "right", Namespace: "default"},
					Spec: networkingv1.NetworkPolicySpec{
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
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "wrong"},
					Spec: networkingv1.NetworkPolicySpec{
						Ingress: []networkingv1.NetworkPolicyIngressRule{
							{
								From: []networkingv1.NetworkPolicyPeer{
									{
										PodSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{"other"},
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
													Values:   []string{"other"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "right", Namespace: "default"},
				Spec: networkingv1.NetworkPolicySpec{
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
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:      "no policy in namespace",
			namespace: "default",
			podLabels: map[string]string{"app": "test"},
			policies:  []*networkingv1.NetworkPolicy{},
			expected:  nil,
			testErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotFound)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{
				Client: fake.NewSimpleClientset(),
			}

			for _, policy := range tc.policies {
				_, err := h.Client.NetworkingV1().NetworkPolicies(tc.namespace).Create(context.Background(), policy, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test policy: %v", err)
				}
			}

			result, err := h.GetPolicyByPodLabels(tc.namespace, tc.podLabels)
			tc.testErr(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

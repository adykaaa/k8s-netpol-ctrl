package attribute

import (
	"context"
	"reflect"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Equal[T comparable](t *testing.T, expected, actual T) {
	t.Helper()

	if expected != actual {
		t.Errorf("want: %v; got: %v", expected, actual)
	}
}

func TestContains(t *testing.T) {
	c1 := Contains([]string{"asd", "asd", "asd"}, "asd")
	Equal(t, c1, true)
	c2 := Contains([]string{"asd", "asd", "asd"}, "dsa")
	Equal(t, c2, false)
	c3 := Contains([]int{1, 2, 3}, 3)
	Equal(t, c3, true)
	c4 := Contains([]int{1, 2, 3}, 4)
	Equal(t, c4, false)
	c5 := Contains([]string{""}, "asd")
	Equal(t, c5, false)
}

func TestMapsEqual(t *testing.T) {
	testCases := []struct {
		name     string
		map1     map[string]string
		map2     map[string]string
		expected bool
	}{
		{
			name:     "both maps are empty",
			map1:     map[string]string{},
			map2:     map[string]string{},
			expected: true,
		},
		{
			name:     "maps have the same content",
			map1:     map[string]string{"foo": "bar"},
			map2:     map[string]string{"foo": "bar"},
			expected: true,
		},
		{
			name:     "maps have different sizes",
			map1:     map[string]string{"foo": "bar"},
			map2:     map[string]string{"foo": "bar", "baz": "qux"},
			expected: false,
		},
		{
			name:     "maps have the same size but different content",
			map1:     map[string]string{"foo": "bar"},
			map2:     map[string]string{"baz": "qux"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MapsEqual(tc.map1, tc.map2)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestAppendUnique(t *testing.T) {
	testCases := []struct {
		name     string
		dst      []string
		src      []string
		expected []string
	}{
		{
			name:     "Both slices are empty",
			dst:      []string{},
			src:      []string{},
			expected: []string{},
		},
		{
			name:     "Destination is empty",
			dst:      []string{},
			src:      []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Source is empty",
			dst:      []string{"a", "b", "c"},
			src:      []string{},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Source and destination have unique items",
			dst:      []string{"a", "b", "c"},
			src:      []string{"d", "e", "f"},
			expected: []string{"a", "b", "c", "d", "e", "f"},
		},
		{
			name:     "Source and destination have overlapping items",
			dst:      []string{"a", "b", "c"},
			src:      []string{"b", "c", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := appendUnique(tc.dst, tc.src)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestMergeLabels(t *testing.T) {
	h := Handler{}
	tests := []struct {
		name        string
		input       []map[string][]string
		expected    map[string][]string
		expectedErr error
	}{
		{
			name: "single map",
			input: []map[string][]string{
				{"app": {"nginx"}},
			},
			expected:    map[string][]string{"app": {"nginx"}},
			expectedErr: nil,
		},
		{
			name: "multiple maps",
			input: []map[string][]string{
				{"app": {"nginx"}},
				{"tier": {"frontend"}},
			},
			expected: map[string][]string{
				"app":  {"nginx"},
				"tier": {"frontend"},
			},
			expectedErr: nil,
		},
		{
			name: "overlapping maps",
			input: []map[string][]string{
				{"app": {"nginx"}},
				{"app": {"redis"}, "tier": {"backend"}},
			},
			expected: map[string][]string{
				"app":  {"nginx", "redis"},
				"tier": {"backend"},
			},
			expectedErr: nil,
		},
		{
			name:        "no maps",
			input:       []map[string][]string{},
			expected:    nil,
			expectedErr: ErrEmptyParam,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.MergeLabels(tt.input...)
			assert.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expected, result)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidLocalEnvVar(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{"my-service.my-namespace.svc.cluster.local", true},
		{"my-pod.my-namespace.pod.cluster.local", true},
		{"invalid-local-env-var", false},
		{"my-service.my-namespace.cluster.local", false},
		{"my-service.my-namespace.svc", false},
		{"my-service.my-namespace.pod", false},
		{"my-service.my-namespace", false},
		{"my-service.svc.cluster.local", false},
		{"my-pod.pod.cluster.local", false},
		{"my-service.my-namespace.svc.cluster.local.extra", false},
		{"my-pod.my-namespace.pod.cluster.local.extra", false},
	}

	for _, c := range cases {
		got := isValidLocalEnvVar(c.input)
		if got != c.expected {
			t.Errorf("isValidLocalEnvVar(%q) == %v, want %v", c.input, got, c.expected)
		}
	}
}

func TestGetLocalEnvVars(t *testing.T) {
	h := Handler{}
	testCases := []struct {
		name     string
		object   metav1.Object
		expected map[string]string
		err      error
	}{
		{
			name: "OK",
			object: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "TEST_ENV",
									Value: "name.namespace.svc.cluster.local",
								},
							},
						},
					},
				},
			},
			expected: map[string]string{"TEST_ENV": "name.namespace.svc.cluster.local"},
			err:      nil,
		},
		{
			name: "returns empty map - has envvar but not cluster local",
			object: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "NOT_CLUSTER_LOCAL",
									Value: "RANDOM",
								},
							},
						},
					},
				},
			},
			expected: map[string]string{},
			err:      nil,
		},
		{
			name:     "returns ErrTypeNotSupported",
			object:   &networkingv1.Ingress{},
			expected: nil,
			err:      ErrTypeNotSupported,
		},
		{
			name:     "returns empty map",
			object:   &corev1.Pod{},
			expected: map[string]string{},
			err:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envVars, err := h.GetLocalEnvVars(tc.object)
			if err != tc.err {
				t.Errorf("expected error %v, got %v", tc.err, err)
			}

			if !MapsEqual(envVars, tc.expected) {
				t.Errorf("expected envVars %v, got %v", tc.expected, envVars)
			}
		})
	}
}

func TestGetLabelsFromEnvVars(t *testing.T) {
	testCases := []struct {
		name     string
		envVars  map[string]string
		pod      corev1.Pod
		svc      corev1.Service
		testErr  func(t *testing.T, err error)
		expected map[string][]string
	}{
		{
			name: "OK - should return POD's labels",
			envVars: map[string]string{
				"TEST_POD": "testpod.default.pod.cluster.local",
			},
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "default",
					Labels: map[string]string{
						"app":   "test",
						"works": "yes",
					},
				},
			},
			svc: corev1.Service{},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			expected: map[string][]string{
				"app":   {"test"},
				"works": {"yes"},
			},
		},
		{
			name: "OK - should return SVC selectors",
			envVars: map[string]string{
				"TEST_SVC": "testsvc.default.svc.cluster.local",
			},
			pod: corev1.Pod{},
			svc: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testsvc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app":   "test",
						"works": "yes",
					},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			expected: map[string][]string{
				"app":   {"test"},
				"works": {"yes"},
			},
		},
		{
			name: "OK - should return both svc selectors and pod labels",
			envVars: map[string]string{
				"TEST_SVC": "testsvc.default.svc.cluster.local",
				"TEST_POD": "testpod.default.pod.cluster.local",
			},
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpod",
					Namespace: "default",
					Labels: map[string]string{
						"app":    "1",
						"obj":    "pod",
						"unique": "label",
					},
				},
			},
			svc: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testsvc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app":     "2",
						"obj":     "svc",
						"unique2": "label",
					},
				},
			},
			testErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			expected: map[string][]string{
				"app":     {"2", "1"},
				"obj":     {"svc", "pod"},
				"unique":  {"label"},
				"unique2": {"label"},
			},
		},
		{
			name:    "should return errNoEnvVars",
			envVars: map[string]string{},
			pod:     corev1.Pod{},
			svc:     corev1.Service{},
			testErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNoEnvVars)
			},
			expected: map[string][]string(nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{
				Client: fake.NewSimpleClientset(),
			}

			_, err := h.Client.CoreV1().Pods(tc.pod.Namespace).Create(context.Background(), &tc.pod, metav1.CreateOptions{})
			if err != nil && !apierrors.IsInvalid(err) {
				t.Fatalf("error during test pod creation %v", err)
			}

			_, err = h.Client.CoreV1().Services(tc.svc.Namespace).Create(context.Background(), &tc.svc, metav1.CreateOptions{})
			if err != nil && !apierrors.IsInvalid(err) {
				t.Fatalf("error during test svc creation %v", err)
			}

			result, err := h.GetLabelsFromEnvVars(tc.envVars)
			tc.testErr(t, err)

			for _, v := range tc.expected {
				sort.Strings(v)
			}

			for _, v := range result {
				sort.Strings(v)
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetLabelsFromSvc(t *testing.T) {
	testCases := []struct {
		name           string
		serviceName    string
		namespace      string
		expectedLabels map[string]string
		createService  bool
		expectErr      bool
	}{
		{
			name:           "valid service",
			serviceName:    "test-svc",
			namespace:      "default",
			expectedLabels: map[string]string{"app": "test-app"},
			createService:  true,
			expectErr:      false,
		},
		{
			name:           "nonexistent service",
			serviceName:    "nonexistent-svc",
			namespace:      "default",
			expectedLabels: map[string]string{},
			createService:  false,
			expectErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{
				Client: fake.NewSimpleClientset(),
			}

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.serviceName,
					Namespace: tc.namespace,
				},
				Spec: corev1.ServiceSpec{
					Selector: tc.expectedLabels,
				},
			}

			if tc.createService {
				_, err := h.Client.CoreV1().Services(tc.namespace).Create(context.Background(), service, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("error injecting service into fake clientset: %v", err)
				}
			}

			labels, err := h.getLabelsFromSvc(tc.serviceName, tc.namespace)

			if (err == nil) == tc.expectErr {
				t.Errorf("expected err == nil to be %v, got %v", tc.expectErr, err == nil)
			}

			if err == nil {
				if len(labels) != len(tc.expectedLabels) {
					t.Errorf("expected %v, got %v", tc.expectedLabels, labels)
				}

				for k, v := range tc.expectedLabels {
					if labels[k] != v {
						t.Errorf("expected %v, got %v", v, labels[k])
					}
				}
			}
		})
	}
}

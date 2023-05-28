package config

import (
	"errors"
	"os"
	"testing"

	"k8s.io/client-go/rest"
)

type MockConfigProvider struct {
	env                map[string]string
	fileExists         bool
	buildConfigError   bool
	inClusterConfigErr bool
}

func (mcp *MockConfigProvider) GetEnv(key string) string {
	return mcp.env[key]
}

func (mcp *MockConfigProvider) Stat(name string) (os.FileInfo, error) {
	if mcp.fileExists {
		return nil, nil
	}
	return nil, errors.New("file not found")
}

func (mcp *MockConfigProvider) BuildConfigFromFlags(masterUrl, kubeconfigPath string) (*rest.Config, error) {
	if mcp.buildConfigError {
		return nil, errors.New("build config error")
	}
	return &rest.Config{}, nil
}

func (mcp *MockConfigProvider) InClusterConfig() (*rest.Config, error) {
	if mcp.inClusterConfigErr {
		return nil, errors.New("in-cluster config error")
	}
	return &rest.Config{}, nil
}

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name               string
		mockConfigProvider *MockConfigProvider
		wantError          error
	}{
		{
			name: "KUBECONFIG env var exists",
			mockConfigProvider: &MockConfigProvider{
				env: map[string]string{
					"KUBECONFIG": "kubeconfig-path",
				},
			},
			wantError: nil,
		},
		{
			name: "KUBECONFIG env var exists, but BuildConfigFromFlags fails",
			mockConfigProvider: &MockConfigProvider{
				env: map[string]string{
					"KUBECONFIG": "kubeconfig-path",
				},
				buildConfigError: true,
			},
			wantError: ErrConfigBuild,
		},
		{
			name: "HOME env var exists, config file exists",
			mockConfigProvider: &MockConfigProvider{
				env: map[string]string{
					"HOME": "home-path",
				},
				fileExists: true,
			},
			wantError: nil,
		},
		{
			name: "HOME env var exists, config file exists, but BuildConfigFromFlags fails",
			mockConfigProvider: &MockConfigProvider{
				env: map[string]string{
					"HOME": "home-path",
				},
				fileExists:       true,
				buildConfigError: true,
			},
			wantError: ErrConfigBuild,
		},
		{
			name: "InClusterConfig is used",
			mockConfigProvider: &MockConfigProvider{
				env: map[string]string{},
			},
			wantError: nil,
		},
		{
			name: "InClusterConfig is used, but InClusterConfig fails",
			mockConfigProvider: &MockConfigProvider{
				env:                map[string]string{},
				inClusterConfigErr: true,
			},
			wantError: ErrConfigNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.mockConfigProvider)
			if err != tt.wantError {
				t.Errorf("NewConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ErrConfigNotFound = errors.New("can't find kubeconfig")
	ErrConfigBuild    = errors.New("could not build kubeconfig from flags")
)

type Provider interface {
	GetEnv(key string) string
	Stat(name string) (os.FileInfo, error)
	BuildConfigFromFlags(masterUrl, kubeconfigPath string) (*rest.Config, error)
	InClusterConfig() (*rest.Config, error)
}

type DefaultProvider struct{}

func (dp *DefaultProvider) GetEnv(key string) string {
	return os.Getenv(key)
}

func (dp *DefaultProvider) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (dp *DefaultProvider) BuildConfigFromFlags(masterUrl, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags(masterUrl, kubeconfigPath)
}

func (dp *DefaultProvider) InClusterConfig() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func New(p Provider) (*rest.Config, error) {
	if kubeconfig := p.GetEnv("KUBECONFIG"); kubeconfig != "" {
		config, err := p.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, ErrConfigBuild
		}
		fmt.Println("using the KUBECONFIG env. var for config")
		return config, nil
	}

	kubeconfigPath := filepath.Join(p.GetEnv("HOME"), ".kube", "config")
	if _, err := p.Stat(kubeconfigPath); err == nil {
		config, err := p.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, ErrConfigBuild
		}
		fmt.Println("using the /home/.kube/config file as config")
		return config, nil
	} else {
		config, err := p.InClusterConfig()
		if err == nil {
			fmt.Println("using the in-cluster service account for kubeconfig")
			return config, nil
		}
	}
	return nil, ErrConfigNotFound
}

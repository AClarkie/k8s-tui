package main

import (
	"fmt"
	"path/filepath"

	"os"

	"github.com/AClarkie/k8s-tui/pkg/controller"
	model "github.com/AClarkie/k8s-tui/pkg/model"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	// Create a new controller
	// Build clientset
	kubeconfig := filepath.Join(homedir, ".kube", "config")
	clientset, err := buildClientset(&kubeconfig)
	if err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	defer close(stop)

	controller := controller.NewController(clientset.AppsV1())
	go func() {
		go controller.Run(stop)
	}()

	model, err := model.InitialModel(controller)
	if err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

}

// buildClientset creates a Kubernetes Clientset, if kubeconfig is empty then
// the in cluster config will attempt to be used.
func buildClientset(kubeconfig *string) (*kubernetes.Clientset, error) {
	var (
		config *rest.Config
		err    error
	)
	// use the current context in kubeconfig
	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config, got err: %s", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to configure k8s client, got err: %w", err)
	}

	return clientset, nil
}

package project

// Kindenv holds the configuration for the kindenv tool.
type Kindenv struct {
	// KubeconfigPath is the path to the kubeconfig file for the kind cluster.
	KubeconfigPath string `json:"kubeconfigPath"`
}

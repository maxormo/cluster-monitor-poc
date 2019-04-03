package provider

type Provider interface {
	RestartNode(node_name string) error
}

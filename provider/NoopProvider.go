package provider

import "fmt"

type noop struct {
}

func (noop noop) RestartNode(node_name string) error {
	fmt.Printf("restarting node %s\n", node_name)
	return nil
}

func NoopProvider() Provider {
	return noop{}
}

package provider

import "fmt"

type Noop struct {
}

func (noop Noop) RestartNode(node_name string) {
	fmt.Printf("restarting node %s\n", node_name)
}

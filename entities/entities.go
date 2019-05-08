package entities

import (
	"time"
)

type Pod struct {
	Name               string
	Node               string
	ReadyCondition     string
	Reason             string
	LastTransitionTime time.Time
}

func (p Pod) String() string {
	return p.Name + " " + p.Node + " " + p.ReadyCondition + " " + p.Reason + "" + p.LastTransitionTime.String()
}

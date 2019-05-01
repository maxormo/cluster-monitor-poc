package entities

import (
	"testing"
)

func TestMetricsClient(t *testing.T) {

	mc := InitMetrics()

	mc.IncHardRestart()
	mc.IncSoftRestart()

}

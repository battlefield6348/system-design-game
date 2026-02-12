package persistence

import (
	"system-design-game/internal/domain/component"
)

// ListAvailableComponents 回傳目前遊戲中可選用的組件列表及其基礎屬性
func ListAvailableComponents() []component.Component {
	return []component.Component{
		{
			ID:              "server-nano",
			Name:            "Nano Server",
			Type:            component.WebServer,
			SetupCost:       50,
			OperationalCost: 0.05,
			Properties: component.Metadata{
				"max_qps":      200,
				"base_latency": 100, // 性能較差，延遲較高
			},
		},
		{
			ID:              "server-standard",
			Name:            "Standard Server",
			Type:            component.WebServer,
			SetupCost:       200,
			OperationalCost: 0.20,
			Properties: component.Metadata{
				"max_qps":      1000,
				"base_latency": 50,
			},
		},
		{
			ID:              "server-high-perf",
			Name:            "High-Perf Server",
			Type:            component.WebServer,
			SetupCost:       800,
			OperationalCost: 0.70,
			Properties: component.Metadata{
				"max_qps":      5000,
				"base_latency": 20,
			},
		},
		{
			ID:              "lb-simple",
			Name:            "Round-Robin LB",
			Type:            component.LoadBalancer,
			SetupCost:       150,
			OperationalCost: 0.10,
			Properties: component.Metadata{
				"max_qps": 20000,
			},
		},
	}
}

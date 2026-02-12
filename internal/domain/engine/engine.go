package engine

import (
	"fmt"
	"system-design-game/internal/domain/component"
	"system-design-game/internal/domain/design"
	"system-design-game/internal/domain/evaluation"
	"system-design-game/internal/domain/scenario"
	"time"
)

// SimpleEngine 是一個基礎的評估引擎實作
type SimpleEngine struct {
	designRepo   design.Repository
	scenarioRepo scenario.Repository
}

func NewSimpleEngine(dr design.Repository, sr scenario.Repository) *SimpleEngine {
	return &SimpleEngine{
		designRepo:   dr,
		scenarioRepo: sr,
	}
}

// Evaluate 實作評估邏輯
func (e *SimpleEngine) Evaluate(designID string) (*evaluation.Result, error) {
	d, err := e.designRepo.GetByID(designID)
	if err != nil {
		return nil, err
	}

	s, err := e.scenarioRepo.GetByID(d.ScenarioID)
	if err != nil {
		return nil, err
	}

	// 流量指標計算：加總所有 WebServer 的 MaxQPS
	var totalCapacity int64
	for _, comp := range d.Components {
		if comp.Type == component.WebServer {
			if maxQPS, ok := comp.Properties["max_qps"].(int64); ok {
				totalCapacity += maxQPS
			} else if maxQPSFloat, ok := comp.Properties["max_qps"].(float64); ok {
				totalCapacity += int64(maxQPSFloat)
			}
		}
	}

	// 取得當前階段流量
	currentQPS := int64(0)
	if len(s.Phases) > 0 {
		currentQPS = s.Phases[0].StartQPS
	}

	// 計算錯誤率與健康度：如果流量超過容量，錯誤率飆升
	errorRate := 0.0
	if currentQPS > totalCapacity && totalCapacity > 0 {
		errorRate = float64(currentQPS-totalCapacity) / float64(currentQPS)
	} else if totalCapacity == 0 && currentQPS > 0 {
		errorRate = 1.0
	}

	health := (1.0 - errorRate) * 100.0

	scores := []evaluation.Score{
		{Dimension: "Capacity", Value: health, Comment: fmt.Sprintf("總容量: %d QPS, 當前流量: %d QPS", totalCapacity, currentQPS)},
		{Dimension: "Availability", Value: health, Comment: "基於容量負載計算"},
	}

	passed := health >= 80

	return &evaluation.Result{
		DesignID:      designID,
		ScenarioID:    s.ID,
		TotalScore:    health,
		Scores:        scores,
		Passed:        passed,
		AvgLatencyMS:  50.0 + (errorRate * 500.0), // 負載越高延遲越高
		ErrorRate:     errorRate,
		TotalQPS:      currentQPS,
		CostPerSec:    0, // 暫時忽略成本
		RevenuePerSec: 0, // 暫時忽略收益
		CreatedAt:     time.Now().Unix(),
	}, nil
}

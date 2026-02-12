package engine

import (
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

	// 基礎指標計算
	var totalCost float64
	for _, comp := range d.Components {
		totalCost += comp.OperationalCost
	}

	// 模擬流量指標 (未來將根據拓撲計算)
	currentQPS := int64(0)
	if len(s.Phases) > 0 {
		currentQPS = s.Phases[0].StartQPS
	}

	revenue := float64(currentQPS) * 0.01 // 簡單獲利模型

	scores := []evaluation.Score{
		{Dimension: "Availability", Value: 85, Comment: "系統運作中"},
		{Dimension: "Profitability", Value: (revenue / (totalCost + 0.1)) * 10, Comment: "收益與成本評估"},
	}

	total := 80.0
	passed := total >= 60

	return &evaluation.Result{
		DesignID:      designID,
		ScenarioID:    s.ID,
		TotalScore:    total,
		Scores:        scores,
		Passed:        passed,
		AvgLatencyMS:  120.5,
		ErrorRate:     0.01,
		TotalQPS:      currentQPS,
		CostPerSec:    totalCost,
		RevenuePerSec: revenue,
		CreatedAt:     time.Now().Unix(),
	}, nil
}

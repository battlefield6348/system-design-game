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

	// TODO: 實作真正的模擬評估算法
	// 這裡先放一個模擬的邏輯
	scores := []evaluation.Score{
		{Dimension: "Availability", Value: 85, Comment: "良好的冗餘設計"},
		{Dimension: "Cost", Value: 70, Comment: "資源利用率尚可優化"},
	}

	total := 77.5
	passed := total >= 60 // 簡易判定

	return &evaluation.Result{
		DesignID:   designID,
		ScenarioID: s.ID,
		TotalScore: total,
		Scores:     scores,
		Passed:     passed,
		CreatedAt:  time.Now().Unix(),
	}, nil
}

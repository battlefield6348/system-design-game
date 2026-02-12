package persistence

import (
	"fmt"
	"system-design-game/internal/domain/scenario"
)

// InMemScenarioRepository 記憶體實作的 Scenario Repository
type InMemScenarioRepository struct {
	scenarios map[string]*scenario.Scenario
}

func NewInMemScenarioRepository() *InMemScenarioRepository {
	repo := &InMemScenarioRepository{
		scenarios: make(map[string]*scenario.Scenario),
	}
	repo.initMockData()
	return repo
}

func (r *InMemScenarioRepository) initMockData() {
	s1 := &scenario.Scenario{
		ID:          "s1",
		Title:       "短網址系統 (TinyURL)",
		Description: "設計一個每秒處理 10k 請求的短網址系統，支援流量從 100 到 10,000 的指數級增長。",
		Goal: scenario.Goal{
			MinQPS:        10000,
			MaxLatencyMS: 200,
			Availability: 99.9,
			Duration:     60, // 持續 60 秒的測試
		},
		Phases: []scenario.TrafficPhase{
			{Name: "初期上線", StartQPS: 100, EndQPS: 1000, DurationSeconds: 60},    // 1 分鐘成長到 1k
			{Name: "口碑傳播", StartQPS: 1000, EndQPS: 5000, DurationSeconds: 120},  // 2 分鐘成長到 5k
			{Name: "流量巔峰", StartQPS: 5000, EndQPS: 10000, DurationSeconds: 300}, // 5 分鐘成長到 10k
		},
	}
	r.scenarios[s1.ID] = s1
}

func (r *InMemScenarioRepository) GetByID(id string) (*scenario.Scenario, error) {
	s, ok := r.scenarios[id]
	if !ok {
		return nil, fmt.Errorf("scenario not found: %s", id)
	}
	return s, nil
}

func (r *InMemScenarioRepository) ListAll() ([]*scenario.Scenario, error) {
	var result []*scenario.Scenario
	for _, s := range r.scenarios {
		result = append(result, s)
	}
	return result, nil
}

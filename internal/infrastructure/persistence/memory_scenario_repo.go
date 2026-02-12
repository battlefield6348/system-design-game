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
		Description: "設計一個每秒處理 10k 請求的短網址系統，確保高可用性。",
		Goal: scenario.Goal{
			MinQPS:        10000,
			MaxLatencyMS: 200,
			Availability: 99.9,
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

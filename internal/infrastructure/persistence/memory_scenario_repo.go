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
		ID:          "tinyurl",
		Title:       "短網址系統 (TinyURL)",
		Description: "設計一個高讀取的短網址系統。挑戰：在極低預算下處理 100k 的跳轉請求，必須善用 Cache。",
		Goal: scenario.Goal{
			MinQPS:        100000,
			MaxLatencyMS: 50,
			Availability: 99.9,
			Duration:     300,
		},
		Phases: []scenario.TrafficPhase{
			{Name: "穩定增長", StartQPS: 1000, EndQPS: 100000, DurationSeconds: 300},
		},
	}

	s2 := &scenario.Scenario{
		ID:          "flash-sale",
		Title:       "快閃搶購 (Flash Sale)",
		Description: "雙 11 搶購活動。挑戰：在 10 秒內應對從 0 到 500k 的突發流量，需使用 MQ 消峰填谷。",
		Goal: scenario.Goal{
			MinQPS:        500000,
			MaxLatencyMS: 500,
			Availability: 95.0,
			Duration:     60,
		},
		Phases: []scenario.TrafficPhase{
			{Name: "熱身", StartQPS: 100, EndQPS: 1000, DurationSeconds: 30},
			{Name: "開賣瞬間", StartQPS: 1000, EndQPS: 500000, DurationSeconds: 10},
			{Name: "餘溫", StartQPS: 500000, EndQPS: 10000, DurationSeconds: 20},
		},
	}

	s3 := &scenario.Scenario{
		ID:          "video-platform",
		Title:       "影音串流 (Netflix/YouTube)",
		Description: "全球化的影音平台。挑戰：降低跨國延遲，提升內容分發效率，需善用 CDN 與 Object Storage。",
		Goal: scenario.Goal{
			MinQPS:        50000,
			MaxLatencyMS: 20,
			Availability: 99.99,
			Duration:     600,
		},
		Phases: []scenario.TrafficPhase{
			{Name: "全球高峰", StartQPS: 5000, EndQPS: 50000, DurationSeconds: 600},
		},
	}

	r.scenarios[s1.ID] = s1
	r.scenarios[s2.ID] = s2
	r.scenarios[s3.ID] = s3
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

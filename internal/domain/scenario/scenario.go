package scenario

// Scenario 代表一個遊戲關卡或挑戰情境
type Scenario struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Goal        Goal           `json:"goal"`
	Phases      []TrafficPhase `json:"phases"` // 指數級增長可以透過多個 phase 組合
	Constraints []Constraint   `json:"constraints"`
}

// Goal 定義關卡目標
type Goal struct {
	MinQPS        int64   `json:"min_qps"`
	MaxLatencyMS int64   `json:"max_latency_ms"`
	Availability float64 `json:"availability"`
	Duration     int     `json:"duration"` // 測試需要持續多久（秒）
}

// TrafficPhase 定義流量成長的一個階段
type TrafficPhase struct {
	Name            string `json:"name"`            // 階段名稱，如 "Normal", "Viral", "DDoS"
	StartQPS        int64  `json:"start_qps"`        // 階段起始 QPS
	EndQPS          int64  `json:"end_qps"`          // 階段結束 QPS (可用於模擬指數或線性增長)
	DurationSeconds int    `json:"duration_seconds"` // 該階段持續秒數
}

// Constraint 定義限制條件，例如：總預算限制
type Constraint struct {
	Type  string `json:"type"`
	Value int64  `json:"value"`
}

// Repository 定義 Scenario 的持久化介面
type Repository interface {
	GetByID(id string) (*Scenario, error)
	ListAll() ([]*Scenario, error)
}

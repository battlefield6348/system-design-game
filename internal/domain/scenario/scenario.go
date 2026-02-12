package scenario

// Scenario 代表一個遊戲關卡或挑戰情境
type Scenario struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Goal        Goal     `json:"goal"`
	Constraints []Constraint `json:"constraints"`
}

// Goal 定義關卡目標，例如：達成特定 QPS 或 可用性
type Goal struct {
	MinQPS        int64   `json:"min_qps"`
	MaxLatencyMS int64   `json:"max_latency_ms"`
	Availability float64 `json:"availability"`
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

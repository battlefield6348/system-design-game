package evaluation

// Score 代表某個維度的評分
type Score struct {
	Dimension string  `json:"dimension"` // 如：Availability, Scalability, Cost, Performance
	Value     float64 `json:"value"`     // 0-100
	Comment   string  `json:"comment"`   // 針對該維度的具體建議
}

// Result 是針對一次系統設計的完整評估報告
type Result struct {
	DesignID   string  `json:"design_id"`
	ScenarioID string  `json:"scenario_id"`
	TotalScore float64 `json:"total_score"`
	Scores     []Score `json:"scores"`
	Passed     bool    `json:"passed"`    // 是否達到關卡目標
	CreatedAt  int64   `json:"created_at"`
}

// Engine 定義評估引擎的介面
type Engine interface {
	Evaluate(designID string) (*Result, error)
}

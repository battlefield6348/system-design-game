package evaluation

// Score 代表某個維度的評分
type Score struct {
	Dimension string  `json:"dimension"` // 如：Availability, Scalability, Cost, Performance
	Value     float64 `json:"value"`     // 0-100
	Comment   string  `json:"comment"`   // 針對該維度的具體建議
}

// Result 是針對系統設計的效能與經濟評估輸出
type Result struct {
	DesignID   string  `json:"design_id"`
	ScenarioID string  `json:"scenario_id"`
	TotalScore float64 `json:"total_score"`
	Scores     []Score `json:"scores"`
	Passed     bool    `json:"passed"`

	// 運行時指標 (Endless mode)
	AvgLatencyMS float64 `json:"avg_latency_ms"`
	ErrorRate    float64 `json:"error_rate"`
	TotalQPS     int64   `json:"total_qps"`
	CostPerSec   float64 `json:"cost_per_sec"`
	RevenuePerSec float64 `json:"revenue_per_sec"`

	CreatedAt int64 `json:"created_at"`
	ActiveComponentIDs  []string `json:"active_component_ids"`  // 實際有接收到流量的組件 ID
	CrashedComponentIDs []string `json:"crashed_component_ids"` // 已經掛掉的組件 ID
}

// Engine 定義評估引擎的介面
type Engine interface {
	Evaluate(designID string, elapsedSeconds int64) (*Result, error)
}

package world

// GameState 代表玩家目前的遊戲狀態（無盡模式核心）
type GameState struct {
	PlayerID     string  `json:"player_id"`
	Balance      float64 `json:"balance"`       // 目前的金幣/預算
	TotalUsers   int64   `json:"total_users"`   // 累積獲得的使用者
	SystemHealth float64 `json:"system_health"` // 0-100，根據延遲與錯誤率計算
	Uptime       int64   `json:"uptime"`        // 系統總運行秒數
	LastTick     int64   `json:"last_tick"`     // 上次計算的時間戳
}

// UpdateMetrics 根據當前系統表現更新遊戲狀態
func (s *GameState) UpdateMetrics(avgLatency float64, errorRate float64, qps int64) {
	// 健康度計算邏輯：延遲越高、錯誤越多，健康度下降
	health := 100.0
	if avgLatency > 500 {
		health -= (avgLatency - 500) / 10
	}
	health -= errorRate * 100

	if health < 0 {
		health = 0
	}
	s.SystemHealth = health

	// 根據成功的 QPS 增加使用者與收益
	successQPS := float64(qps) * (1.0 - errorRate)
	s.Balance += successQPS * 0.01 // 假設每 100 個成功請求賺 1 元
	s.TotalUsers += int64(successQPS)
}

// DeductCost 扣除運作成本
func (s *GameState) DeductCost(costPerSecond float64, durationSeconds int64) {
	s.Balance -= costPerSecond * float64(durationSeconds)
	s.Uptime += durationSeconds
}

// Repository 定義狀態存取介面
type Repository interface {
	Save(state *GameState) error
	GetByPlayerID(playerID string) (*GameState, error)
}

package design

import "system-design-game/internal/domain/component"

// Connection 定義組件之間的連通性
type Connection struct {
	FromID      string `json:"from_id"`
	ToID        string `json:"to_id"`
	Protocol    string `json:"protocol"`     // 如：HTTP, GPRC, TCP
	TrafficType string `json:"traffic_type"` // "all", "read", "write"
}

// Design 代表玩家設計的完整系統拓撲
type Design struct {
	ID          string                `json:"id"`
	PlayerID    string                `json:"player_id"`
	ScenarioID  string                `json:"scenario_id"`
	Components  []component.Component `json:"components"`
	Connections []Connection          `json:"connections"`
	Properties  component.Metadata    `json:"properties"` // 全域屬性，如：使用者留存率
	CreatedAt   int64                 `json:"created_at"`
	UpdatedAt   int64                 `json:"updated_at"`
}

// Repository 定義 Design 的持久化介面
type Repository interface {
	Save(design *Design) error
	GetByID(id string) (*Design, error)
	ListByPlayerID(playerID string) ([]*Design, error)
}

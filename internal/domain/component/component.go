package component

// Type 定義組件類型
type Type string

const (
	TrafficSource Type = "TRAFFIC_SOURCE"
	LoadBalancer  Type = "LOAD_BALANCER"
	WebServer     Type = "WEB_SERVER"
	Database      Type = "DATABASE"
	Cache         Type = "CACHE"
	MessageQueue  Type = "MESSAGE_QUEUE"
	CDN           Type = "CDN"
	WAF           Type = "WAF"
	ObjectStorage Type = "OBJECT_STORAGE"
	SearchEngine  Type = "SEARCH_ENGINE"
)

// Component 代表系統中的一個最小單位
type Component struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            Type     `json:"type"`
	SetupCost       float64  `json:"setup_cost"`       // 購買/建立成本
	OperationalCost float64  `json:"operational_cost"` // 每秒運作成本
	Properties      Metadata `json:"properties"`       // 儲存組件的具體參數（如：記憶體大小、連線數限制）
}

// Metadata 儲存組件的屬性，例如：
// "max_qps": 1000
// "base_latency": 50
// "max_connections": 500
type Metadata map[string]interface{}

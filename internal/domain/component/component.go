package component

// Type 定義組件類型
type Type string

const (
	LoadBalancer Type = "LOAD_BALANCER"
	WebServer    Type = "WEB_SERVER"
	Database     Type = "DATABASE"
	Cache        Type = "CACHE"
	MessageQueue Type = "MESSAGE_QUEUE"
)

// Component 代表系統中的一個最小單位
type Component struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        Type     `json:"type"`
	Properties  Metadata `json:"properties"` // 儲存組件的具體參數（如：記憶體大小、連線數限制）
}

// Metadata 儲存組件的屬性
type Metadata map[string]interface{}

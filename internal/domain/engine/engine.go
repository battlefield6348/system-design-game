package engine

import (
	"fmt"
	"system-design-game/internal/domain/component"
	"system-design-game/internal/domain/design"
	"system-design-game/internal/domain/evaluation"
	"system-design-game/internal/domain/scenario"
	"time"
)

// SimpleEngine 是一個基礎的評估引擎實作
type SimpleEngine struct {
	designRepo   design.Repository
	scenarioRepo scenario.Repository
}

func NewSimpleEngine(dr design.Repository, sr scenario.Repository) *SimpleEngine {
	return &SimpleEngine{
		designRepo:   dr,
		scenarioRepo: sr,
	}
}

// Evaluate 實作評估邏輯
func (e *SimpleEngine) Evaluate(designID string, elapsedSeconds int64) (*evaluation.Result, error) {
	d, err := e.designRepo.GetByID(designID)
	if err != nil {
		return nil, err
	}

	s, err := e.scenarioRepo.GetByID(d.ScenarioID)
	if err != nil {
		return nil, err
	}

	// 1. 建立連線地圖 (Adjacency List)
	adj := make(map[string][]string)
	for _, conn := range d.Connections {
		adj[conn.FromID] = append(adj[conn.FromID], conn.ToID)
	}

	// 2. 找出所有流量起點 (TrafficSource)
	var roots []string
	compMap := make(map[string]component.Component)
	for _, comp := range d.Components {
		compMap[comp.ID] = comp
		if comp.Type == component.TrafficSource {
			roots = append(roots, comp.ID)
		}
	}

	// 3. 遍歷圖形，找出各種單位的有效集合
	visited := make(map[string]bool)
	reachableWebServers := make(map[string]bool)
	reachableDatabases := make(map[string]bool)
	reachableCaches := make(map[string]bool)
	crashedNodes := make(map[string]bool)
	
	var traverse func(string)
	traverse = func(id string) {
		comp, exists := compMap[id]
		if !exists || visited[id] {
			return
		}

		// 檢查持久性崩潰標記 (必須由外部操作重啟)
		if crashed, ok := comp.Properties["crashed"].(bool); ok && crashed {
			return
		}
		
		visited[id] = true
		if comp.Type == component.WebServer {
			reachableWebServers[id] = true
		}
		if comp.Type == component.Database {
			reachableDatabases[id] = true
		}
		if comp.Type == component.Cache {
			reachableCaches[id] = true
		}
		
		for _, nextID := range adj[id] {
			traverse(nextID)
		}
	}

	// 執行初始遍歷
	for _, root := range roots {
		traverse(root)
	}

	// 4. 計算各層級容量
	getCompMaxQPS := func(id string) int64 {
		comp := compMap[id]
		if maxQPS, ok := comp.Properties["max_qps"].(int64); ok {
			return maxQPS
		} else if maxQPSFloat, ok := comp.Properties["max_qps"].(float64); ok {
			return int64(maxQPSFloat)
		}
		return 0
	}

	// 根據經過時間計算當前應有的 QPS
	var currentQPS int64
	tempElapsed := elapsedSeconds
	for _, phase := range s.Phases {
		if tempElapsed < int64(phase.DurationSeconds) {
			progress := float64(tempElapsed) / float64(phase.DurationSeconds)
			currentQPS = phase.StartQPS + int64(float64(phase.EndQPS-phase.StartQPS)*progress)
			break
		}
		tempElapsed -= int64(phase.DurationSeconds)
		currentQPS = phase.EndQPS
	}

	// 操爆機制預判：計算各節點的預估負載
	for id, comp := range compMap {
		if comp.Type == component.TrafficSource {
			continue
		}
		
		maxQPS := int64(0)
		if v, ok := comp.Properties["max_qps"].(int64); ok {
			maxQPS = v
		} else if v, ok := comp.Properties["max_qps"].(float64); ok {
			maxQPS = int64(v)
		}

		if maxQPS > 0 {
			load := int64(0)
			if comp.Type == component.Database || comp.Type == component.Cache || comp.Type == component.LoadBalancer {
				load = currentQPS
			} else if comp.Type == component.WebServer {
				serverCount := int64(0)
				for _, c := range d.Components {
					if c.Type == component.WebServer {
						serverCount++
					}
				}
				if serverCount > 0 {
					load = currentQPS / serverCount
				}
			}

			if load > int64(float64(maxQPS)*1.5) {
				crashedNodes[id] = true
			}
		}
	}

	// 重新執行尋路 (考慮崩潰節點的可達性)
	visited = make(map[string]bool)
	reachableWebServers = make(map[string]bool)
	reachableDatabases = make(map[string]bool)
	reachableCaches = make(map[string]bool)
	for _, root := range roots {
		traverse(root)
	}

	// 重新計算有效容量
	var serverCapacity int64
	for id := range reachableWebServers {
		serverCapacity += getCompMaxQPS(id)
	}
	var dbCapacity int64
	for id := range reachableDatabases {
		dbCapacity += getCompMaxQPS(id)
	}

	// 實作快取減壓效果：如果有快取，模擬 80% 的命中率
	effectiveDBCapacity := dbCapacity
	if len(reachableCaches) > 0 {
		effectiveDBCapacity = dbCapacity * 5
	}

	totalCapacity := serverCapacity
	if len(reachableDatabases) > 0 && effectiveDBCapacity < totalCapacity {
		totalCapacity = effectiveDBCapacity
	}
	if len(reachableWebServers) == 0 {
		totalCapacity = 0
	}

	// 計算錯誤率
	errorRate := 0.0
	if currentQPS > totalCapacity && totalCapacity > 0 {
		errorRate = float64(currentQPS-totalCapacity) / float64(currentQPS)
	} else if (totalCapacity == 0 || len(roots) == 0) && currentQPS > 0 {
		errorRate = 1.0
	}

	health := (1.0 - errorRate) * 100.0

	bottleneck := "伺服器"
	if len(reachableDatabases) > 0 && effectiveDBCapacity < serverCapacity {
		bottleneck = "資料庫"
	}

	scores := []evaluation.Score{
		{Dimension: "Topology", Value: float64(len(reachableWebServers)) * 10, Comment: fmt.Sprintf("有效連接伺服器: %d 台", len(reachableWebServers))},
		{Dimension: "Capacity", Value: health, Comment: fmt.Sprintf("當前系統瓶頸: %s (上限 %d QPS, 需求 %d QPS)", bottleneck, totalCapacity, currentQPS)},
	}

	activeIDs := make([]string, 0)
	for id := range visited {
		activeIDs = append(activeIDs, id)
	}

	crashedIDs := make([]string, 0)
	for id := range crashedNodes {
		crashedIDs = append(crashedIDs, id)
	}

	passed := health >= 80

	return &evaluation.Result{
		DesignID:      designID,
		ScenarioID:    s.ID,
		TotalScore:    health,
		Scores:        scores,
		Passed:        passed,
		AvgLatencyMS:  50.0 + (errorRate * 500.0),
		ErrorRate:     errorRate,
		TotalQPS:      currentQPS,
		CreatedAt:     time.Now().Unix(),
		ActiveComponentIDs:  activeIDs,
		CrashedComponentIDs: crashedIDs,
	}, nil
}

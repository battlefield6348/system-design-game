package engine

import (
	"fmt"
	"math"
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

	// 2. 找出所有流量起點
	var roots []string
	compMap := make(map[string]component.Component)
	for _, comp := range d.Components {
		compMap[comp.ID] = comp
		if comp.Type == component.TrafficSource {
			roots = append(roots, comp.ID)
		}
	}

	// 3. 獲取當前應有的 QPS
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
	if currentQPS == 0 && len(s.Phases) > 0 {
		currentQPS = s.Phases[len(s.Phases)-1].EndQPS
	}

	// 4. 遍歷圖形，計算每個組件的實際 Load
	visited := make(map[string]bool)
	reachableWebServers := make(map[string]bool)
	reachableDatabases := make(map[string]bool)
	reachableCaches := make(map[string]bool)
	crashedNodes := make(map[string]bool)
	compLoads := make(map[string]int64)

	// 先估算伺服器總數
	serverCount := int64(0)
	for _, c := range d.Components {
		if c.Type == component.WebServer {
			serverCount++
		}
	}

	var simulateFlow func(string, int64)
	simulateFlow = func(id string, inputQPS int64) {
		comp, exists := compMap[id]
		if !exists || visited[id] {
			return
		}

		// 檢查持久性崩潰標記
		if crashed, ok := comp.Properties["crashed"].(bool); ok && crashed {
			crashedNodes[id] = true
			return
		}
		
		maxQPS := int64(0)
		if v, ok := comp.Properties["max_qps"].(int64); ok {
			maxQPS = v
		} else if v, ok := comp.Properties["max_qps"].(float64); ok {
			maxQPS = int64(v)
		}

		// 判斷是否「新發生」崩潰 (負載過重 > 150%)
		if maxQPS > 0 && inputQPS > int64(float64(maxQPS)*1.5) {
			crashedNodes[id] = true
			return
		}

		visited[id] = true
		compLoads[id] = inputQPS

		outputQPS := inputQPS
		if comp.Type == component.WebServer {
			reachableWebServers[id] = true
		} else if comp.Type == component.Database {
			reachableDatabases[id] = true
		} else if comp.Type == component.Cache {
			reachableCaches[id] = true
			outputQPS = int64(float64(inputQPS) * 0.2) // 80% Cache Hit
		}

		for _, nextID := range adj[id] {
			simulateFlow(nextID, outputQPS)
		}
	}

	for _, root := range roots {
		compLoads[root] = currentQPS
		visited[root] = true
		for _, firstID := range adj[root] {
			initialQPS := currentQPS
			if compMap[firstID].Type == component.WebServer && serverCount > 0 {
				initialQPS = currentQPS / serverCount
			}
			simulateFlow(firstID, initialQPS)
		}
	}

	// 5. 計算總容量與系統擁塞因子 (用於 Latency 模擬)
	var serverCapacity int64
	for id := range reachableWebServers {
		serverCapacity += getCompMaxQPS(compMap[id])
	}
	var dbCapacity int64
	for id := range reachableDatabases {
		dbCapacity += getCompMaxQPS(compMap[id])
	}

	// 擁塞因子模擬：如果有組件負載 > 80%，延遲開始上升
	congestionFactor := 1.0
	for id, load := range compLoads {
		maxQPS := getCompMaxQPS(compMap[id])
		if maxQPS > 0 {
			utilization := float64(load) / float64(maxQPS)
			if utilization > 0.8 {
				// M/M/1 排隊理論簡化版：1 / (1 - utilization)
				factor := 1.0 / (1.1 - utilization)
				if factor > congestionFactor {
					congestionFactor = factor
				}
			}
		}
	}

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

	// 6. 計算指標
	errorRate := 0.0
	if currentQPS > totalCapacity && totalCapacity > 0 {
		errorRate = float64(currentQPS-totalCapacity) / float64(currentQPS)
	} else if (totalCapacity == 0 || len(roots) == 0) && currentQPS > 0 {
		errorRate = 1.0
	}
	health := (1.0 - errorRate) * 100.0

	// 延遲模擬：基礎 50ms * 擁塞因子
	avgLatency := 50.0 * congestionFactor
	if avgLatency > 2000.0 {
		avgLatency = 2000.0 // 上限 2 秒
	}

	bottleneck := "伺服器"
	if len(reachableDatabases) > 0 && effectiveDBCapacity < serverCapacity {
		bottleneck = "資料庫"
	}

	scores := []evaluation.Score{
		{Dimension: "Topology", Value: float64(len(reachableWebServers)) * 10, Comment: fmt.Sprintf("活躍伺服器: %d 台", len(reachableWebServers))},
		{Dimension: "Capacity", Value: health, Comment: fmt.Sprintf("瓶頸: %s (上限 %d, 需求 %d QPS)", bottleneck, totalCapacity, currentQPS)},
		{Dimension: "Performance", Value: math.Max(0, 100-(avgLatency-50)/5), Comment: fmt.Sprintf("平均延遲: %.1f ms", avgLatency)},
	}

	activeIDs := make([]string, 0, len(visited))
	for id := range visited {
		activeIDs = append(activeIDs, id)
	}
	crashedIDs := make([]string, 0, len(crashedNodes))
	for id := range crashedNodes {
		crashedIDs = append(crashedIDs, id)
	}

	return &evaluation.Result{
		DesignID:      designID,
		ScenarioID:    s.ID,
		TotalScore:    health,
		Scores:        scores,
		Passed:        health >= 80,
		AvgLatencyMS:  avgLatency,
		ErrorRate:     errorRate,
		TotalQPS:      currentQPS,
		CreatedAt:     time.Now().Unix(),
		ActiveComponentIDs:  activeIDs,
		CrashedComponentIDs: crashedIDs,
		ComponentLoads:      compLoads,
	}, nil
}

func getCompMaxQPS(comp component.Component) int64 {
	if v, ok := comp.Properties["max_qps"].(int64); ok {
		return v
	} else if v, ok := comp.Properties["max_qps"].(float64); ok {
		return int64(v)
	}
	return 0
}

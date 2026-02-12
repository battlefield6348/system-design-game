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

	// 先估算伺服器總數，用於平攤流量
	serverCount := int64(0)
	for _, c := range d.Components {
		if c.Type == component.WebServer {
			serverCount++
		}
	}

	// 核心模擬邏輯：遞迴計算流量在不同層級的衰減
	var simulateFlow func(string, int64)
	simulateFlow = func(id string, inputQPS int64) {
		if visited[id] || crashedNodes[id] {
			return
		}
		
		comp := compMap[id]
		maxQPS := int64(0)
		if v, ok := comp.Properties["max_qps"].(int64); ok {
			maxQPS = v
		} else if v, ok := comp.Properties["max_qps"].(float64); ok {
			maxQPS = int64(v)
		}

		// 判斷是否崩潰 (負載過重)
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
			// 關鍵：Cache 會吸收 80% 的讀取請求，只有 20% 會往後傳給 DB
			outputQPS = int64(float64(inputQPS) * 0.2)
		}

		for _, nextID := range adj[id] {
			simulateFlow(nextID, outputQPS)
		}
	}

	for _, root := range roots {
		// 流量來源自身不消耗 QPS
		compLoads[root] = currentQPS
		visited[root] = true
		for _, firstID := range adj[root] {
			// 如果流量來源接的是 WebServer，則平攤流量
			initialQPS := currentQPS
			if compMap[firstID].Type == component.WebServer && serverCount > 0 {
				initialQPS = currentQPS / serverCount
			}
			simulateFlow(firstID, initialQPS)
		}
	}

	// 5. 計算總容量 (用於健康度評估)
	var serverCapacity int64
	for id := range reachableWebServers {
		comp := compMap[id]
		if v, ok := comp.Properties["max_qps"].(int64); ok {
			serverCapacity += v
		} else if v, ok := comp.Properties["max_qps"].(float64); ok {
			serverCapacity += int64(v)
		}
	}
	var dbCapacity int64
	for id := range reachableDatabases {
		comp := compMap[id]
		if v, ok := comp.Properties["max_qps"].(int64); ok {
			dbCapacity += v
		} else if v, ok := comp.Properties["max_qps"].(float64); ok {
			dbCapacity += int64(v)
		}
	}

	// 如果有 Cache 存在於路徑中，提升 DB 的「效應容量」
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

	// 6. 計算錯誤率與健康度
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
		{Dimension: "Capacity", Value: health, Comment: fmt.Sprintf("瓶頸: %s (有效上限 %d, 當前 %d QPS)", bottleneck, totalCapacity, currentQPS)},
	}

	activeIDs := make([]string, 0)
	for id := range visited {
		activeIDs = append(activeIDs, id)
	}
	crashedIDs := make([]string, 0)
	for id := range crashedNodes {
		crashedIDs = append(crashedIDs, id)
	}

	return &evaluation.Result{
		DesignID:      designID,
		ScenarioID:    s.ID,
		TotalScore:    health,
		Scores:        scores,
		Passed:        health >= 80,
		AvgLatencyMS:  50.0 + (errorRate * 500.0),
		ErrorRate:     errorRate,
		TotalQPS:      currentQPS,
		CreatedAt:     time.Now().Unix(),
		ActiveComponentIDs:  activeIDs,
		CrashedComponentIDs: crashedIDs,
		ComponentLoads:      compLoads,
	}, nil
}

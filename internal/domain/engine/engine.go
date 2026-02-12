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

	// 2. 找出所有組件與流量起點
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
	if currentQPS <= 0 && len(s.Phases) > 0 {
		currentQPS = s.Phases[len(s.Phases)-1].EndQPS
	}

	// 4. 核心物理流量模擬：計算負載與截斷
	visited := make(map[string]bool)
	crashedNodes := make(map[string]bool)
	compLoads := make(map[string]int64)             // 紀錄組件收到的「總輸入流量」
	componentProcessedQPS := make(map[string]int64) // 紀錄組件「成功處理並往下傳」的流量

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

		maxQPS := getCompMaxQPS(comp)


		// 判斷是否新發生崩潰 (1.5 倍負載)
		// 加入 Grace Period 機制：如果剛重啟不到 5 秒，則暫時免疫崩潰
		// 這模擬了系統冷啟動時的保護機制，或是為了讓維運人員有時間處理
		isGracePeriod := false
		if restartedAt, ok := comp.Properties["restartedAt"].(float64); ok {
			// restartedAt 是前端傳來的 gameTime (秒)
			// 我們比較 elapsedSeconds 與 restartedAt 的差距
			if float64(elapsedSeconds)-restartedAt < 5.0 {
				isGracePeriod = true
			}
		} else if restartedAt, ok := comp.Properties["restartedAt"].(int64); ok {
			if elapsedSeconds-restartedAt < 5 {
				isGracePeriod = true
			}
		}

		if !isGracePeriod && maxQPS > 0 && inputQPS > int64(float64(maxQPS)*1.5) {
			crashedNodes[id] = true
			return
		}

		visited[id] = true
		compLoads[id] = inputQPS

		// 實作截斷邏輯：成功處理的流量不能超過組件上限
		processedQPS := inputQPS
		if maxQPS > 0 && inputQPS > maxQPS {
			processedQPS = maxQPS // 剩餘流量在這裡「死掉」了，不往後傳
		}
		componentProcessedQPS[id] = processedQPS

		outputQPS := processedQPS
		if comp.Type == component.Cache {
			// 快取效果：假設 80% 命中的請求在這一層就成功返回，不往後傳
			outputQPS = int64(float64(processedQPS) * 0.2)
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

	// 5. 計算總體健康度 (以最終成功到達所有終點的流量比例來計算)
	// 重新計算有效容量以評估指標
	var serverCapacity int64
	for id, comp := range compMap {
		if comp.Type == component.WebServer && visited[id] {
			serverCapacity += getCompMaxQPS(comp)
		}
	}
	var dbCapacity int64
	hasCache := false
	for id, comp := range compMap {
		if comp.Type == component.Database && visited[id] {
			dbCapacity += getCompMaxQPS(comp)
		}
		if comp.Type == component.Cache && visited[id] {
			hasCache = true
		}
	}

	effectiveDBCapacity := dbCapacity
	if hasCache {
		effectiveDBCapacity = dbCapacity * 5
	}

	systemCapacity := serverCapacity
	if dbCapacity > 0 && effectiveDBCapacity < systemCapacity {
		systemCapacity = effectiveDBCapacity
	}

	errorRate := 0.0
	if currentQPS > systemCapacity && systemCapacity > 0 {
		errorRate = float64(currentQPS-systemCapacity) / float64(currentQPS)
	} else if systemCapacity == 0 && currentQPS > 0 {
		errorRate = 1.0
	}
	health := (1.0 - errorRate) * 100.0

	// 延遲模擬
	congestionFactor := 1.0
	for id, load := range compLoads {
		maxQPS := getCompMaxQPS(compMap[id])
		if maxQPS > 0 {
			utilization := float64(load) / float64(maxQPS)
			if utilization > 0.8 {
				factor := 1.0 / math.Max(0.01, 1.1-utilization)
				if factor > congestionFactor {
					congestionFactor = factor
				}
			}
		}
	}
	avgLatency := 50.0 * congestionFactor
	if avgLatency > 2000 {
		avgLatency = 2000
	}

	scores := []evaluation.Score{
		{Dimension: "Capacity", Value: health, Comment: fmt.Sprintf("健康度: %.1f%% (系統容量 %d QPS)", health, systemCapacity)},
		{Dimension: "Performance", Value: math.Max(0, 100-(avgLatency-50)/10), Comment: fmt.Sprintf("平均延遲: %.1f ms", avgLatency)},
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
		DesignID:            designID,
		ScenarioID:          s.ID,
		TotalScore:          health,
		Scores:              scores,
		Passed:              health >= 80,
		AvgLatencyMS:        avgLatency,
		ErrorRate:           errorRate,
		TotalQPS:            currentQPS,
		CreatedAt:           time.Now().Unix(),
		ActiveComponentIDs:  activeIDs,
		CrashedComponentIDs: crashedIDs,
		ComponentLoads:      compLoads, // 前端會用這個來顯示每個 node 的 load
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

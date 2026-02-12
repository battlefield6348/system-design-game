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
	var baseQPS int64
	for _, comp := range d.Components {
		if comp.Type == component.TrafficSource {
			if v, ok := comp.Properties["start_qps"].(float64); ok {
				baseQPS = int64(v)
			} else if v, ok := comp.Properties["start_qps"].(int64); ok {
				baseQPS = v
			}
		}
	}

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
	
	// 加上使用者設定的初始流量
	currentQPS += baseQPS

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

	var simulateFlow func(string, int64, map[string]bool)
	simulateFlow = func(id string, inputQPS int64, pathVisited map[string]bool) {
		comp, exists := compMap[id]
		if !exists {
			return
		}
		
		// 防止環路 (Cycle Detection)
		if pathVisited[id] {
			return
		}
		newPathVisited := make(map[string]bool)
		for k, v := range pathVisited {
			newPathVisited[k] = v
		}
		newPathVisited[id] = true

		// 檢查持久性崩潰標記
		if crashed, ok := comp.Properties["crashed"].(bool); ok && crashed {
			crashedNodes[id] = true
			return
		}

		maxQPS := getCompMaxQPS(comp)

		// 累加流量到該節點
		compLoads[id] += inputQPS
		totalInputQPS := compLoads[id] // 使用當前累積的總流量來判斷崩潰

		// 判斷是否新發生崩潰 (1.5 倍負載)
		// 加入 Grace Period 機制
		isGracePeriod := false
		if restartedAt, ok := comp.Properties["restartedAt"].(float64); ok {
			if float64(elapsedSeconds)-restartedAt < 5.0 {
				isGracePeriod = true
			}
		} else if restartedAt, ok := comp.Properties["restartedAt"].(int64); ok {
			if elapsedSeconds-restartedAt < 5 {
				isGracePeriod = true
			}
		}

		if !isGracePeriod && maxQPS > 0 && totalInputQPS > int64(float64(maxQPS)*1.5) {
			crashedNodes[id] = true
			// 崩潰後不再往下傳遞流量 (雖然已經累加了 Input，但 Output 為 0)
			return
		}

		visited[id] = true
		
		// 實作截斷邏輯：每個來源的流量被截斷後再往下傳？
		// 這裡有個邏輯難點：如果要累加 Load，必須等所有上游都計算完？
		// 簡單模型：我們不等待，直接將「這一次的 Input」進行截斷後傳遞。
		// 雖然多次呼叫會導致下游被多次觸發，但只要下游也是累加模式就可以。
		
		processedQPS := inputQPS
		// 如果當前累積流量已經超過 MaxQPS，則新來的流量可能全被丟棄
		// 或者：我們簡單地按比例截斷單次 Input
		if maxQPS > 0 && totalInputQPS > maxQPS {
			// 這裡的邏輯比較複雜，簡單做：
			// 如果已經過載，新來的流量視為溢出
			// 但為了讓後端能收到滿載的流量 (2000)，我們至少要讓總 Output 達到 MaxQPS
			// 暫時簡化：每次 Input 都依賴自身的大小做截斷測試 (不完美但可用)
			if inputQPS > maxQPS {
				processedQPS = maxQPS
			}
		}
		componentProcessedQPS[id] = processedQPS

		outputQPS := processedQPS
		if comp.Type == component.Cache {
			// 快取效果：假設 80% 命中的請求在這一層就成功返回，不往後傳
			outputQPS = int64(float64(processedQPS) * 0.2)
		}

		// 分流邏輯 (Load Balancing)
		// 如果有多個下游組件，則平均分配流量
		// 注意：這裡只考慮「同層級分流」，例如 LB -> 3 台 WebServer
		// 如果組件連接到不同類型的後端 (例如同時連 Cache 和 DB)，通常是 Cache 擋前面，DB 在後
		// 但目前的 adjacency list 是平鋪的。我們簡單做：將流量平均分給所有下游
		// 更精確的做法應該是依 component type 分組，但目前簡化處理
		downstreamCount := int64(len(adj[id]))
		if downstreamCount > 0 {
			splitQPS := outputQPS / downstreamCount
			for _, nextID := range adj[id] {
				simulateFlow(nextID, splitQPS, newPathVisited)
			}
		}
	}

	for _, root := range roots {
		compLoads[root] = currentQPS
		visited[root] = true
		// 流量來源也要進行分流，如果連到多個入口 (例如多個 LB)
		simulateFlow(root, currentQPS, make(map[string]bool))
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

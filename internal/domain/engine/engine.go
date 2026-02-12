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
	var isBurstActive bool
	for _, comp := range d.Components {
		if comp.Type == component.TrafficSource {
			if v, ok := comp.Properties["start_qps"].(float64); ok {
				baseQPS = int64(v)
			} else if v, ok := comp.Properties["start_qps"].(int64); ok {
				baseQPS = v
			}
			
			// 處理突發流量 (Burst)
			if burst, ok := comp.Properties["burst_traffic"].(bool); ok && burst {
				// 每 10 秒會有一次 5 倍流量的突發，持續 3 秒
				if elapsedSeconds%10 < 3 {
					baseQPS *= 5
					isBurstActive = true
				}
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
	totalBaseQPS := currentQPS + baseQPS

	// 加上隨機波動 (Fluctuation)
	// 使用 Sine 波模擬自然波動 (±5%)
	fluctuation := 1.0 + 0.05*math.Sin(float64(elapsedSeconds)/5.0)

	// 隨機驟降事件 (Unknown random drops)
	isRandomDrop := false
	// 每 15 秒判定一次，有 10% 機率發生 40% 的驟降，持續 3 秒
	if (elapsedSeconds/15)%10 == 7 && elapsedSeconds%15 < 3 {
		fluctuation *= 0.6
		isRandomDrop = true
	}

	// 使用者留存率 (User Churn / Retention)
	retentionRate := 1.0
	if d.Properties != nil {
		if v, ok := d.Properties["retention_rate"].(float64); ok {
			retentionRate = v
		}
	}

	// 最終實際流量
	currentQPS = int64(float64(totalBaseQPS) * fluctuation * retentionRate)

	// 4. 核心物理流量模擬：計算負載與截斷
	visited := make(map[string]bool)
	crashedNodes := make(map[string]bool)
	compLoads := make(map[string]int64)             // 紀錄組件收到的「總輸入流量」
	compEffectiveMaxQPS := make(map[string]int64)   // 紀錄組件當前的「有效最大處理能力」(含 Auto Scaling)

	compReplicas := make(map[string]int)
	for _, c := range d.Components {
		compReplicas[c.ID] = 1
	}

	var totalFulfilledQPS int64

	// Pass 1: 計算潛在總負載 (Potential Load)
	// 這一步只累加流量，不進行截斷，也不觸發崩潰邏輯
	// 目的：讓每個節點知道自己「將會」收到多少流量
	passesInputLoad := make(map[string]int64)
	
	var calculateLoad func(string, int64, map[string]bool)
	calculateLoad = func(id string, input int64, pathVisited map[string]bool) {
		if _, exists := compMap[id]; !exists {
			return
		}
		if pathVisited[id] {
			return
		}
		newPathVisited := make(map[string]bool)
		for k, v := range pathVisited {
			newPathVisited[k] = v
		}
		newPathVisited[id] = true

		passesInputLoad[id] += input

		// 簡單分流預算傳遞
		downstreamCount := int64(len(adj[id]))
		if downstreamCount > 0 {
			// Cache 邏輯在預算階段也要考慮嗎？
			// 是的，因為 Cache 會減少往下游的「需求」。
			comp := compMap[id]
			output := input
			if comp.Type == component.Cache || comp.Type == component.CDN {
				output = int64(float64(input) * 0.2)
			}
			
			split := output / downstreamCount
			for _, nextID := range adj[id] {
				calculateLoad(nextID, split, newPathVisited)
			}
		}
	}

	for _, root := range roots {
		// 初始流量也需要分流 logic? 
		// 假設 TrafficSource 本身不消耗，直接往下傳
		// 為了對齊，我們把 TrafficSource 當作一個透明節點
		calculateLoad(root, currentQPS, make(map[string]bool))
	}


	// Pass 2: 實際流量傳播 (Actual Flow Propagation)
	// 根據 Pass 1 的 total load 決定崩潰與截斷
	var propagateFlow func(string, int64, map[string]bool)
	propagateFlow = func(id string, input int64, pathVisited map[string]bool) {
		comp, exists := compMap[id]
		if !exists {
			return
		}
		
		if pathVisited[id] {
			return
		}
		newPathVisited := make(map[string]bool)
		for k, v := range pathVisited {
			newPathVisited[k] = v
		}
		newPathVisited[id] = true

		// 檢查持久性崩潰
		if crashed, ok := comp.Properties["crashed"].(bool); ok && crashed {
			crashedNodes[id] = true
			return
		}

		baseMaxQPS := getCompMaxQPS(comp)
		currentMaxQPS := baseMaxQPS

		// 這裡使用 Pass 1 計算出的 "潛在總流量" 來判斷是否崩潰
		// 因為崩潰是看「嘗試打進來的量」，而不是「成功擠進來的量」
		potentialTotalLoad := passesInputLoad[id]
		compLoads[id] = potentialTotalLoad // 前端顯示的是「嘗試請求量」

		// Auto Scaling Logic (Shared by WebServer and AutoScalingGroup)
		if comp.Type == component.WebServer || comp.Type == component.AutoScalingGroup {
			if auto, ok := comp.Properties["auto_scaling"].(bool); ok && auto {
				maxReplicas := 5
				if v, ok := comp.Properties["max_replicas"].(float64); ok {
					maxReplicas = int(v)
				}
				
				threshold := 0.7 // 預設 70% 負載就擴展
				if v, ok := comp.Properties["scale_up_threshold"].(float64); ok {
					threshold = v / 100.0
				}

				warmup := int64(10) // 預設 10 秒暖機
				if v, ok := comp.Properties["warmup_seconds"].(float64); ok {
					warmup = int64(v)
				}

				// 計算目標副本數 (Target Replicas)
				// 當前的潛在負載 / (單機能力 * 門檻)
				needed := float64(potentialTotalLoad) / (float64(baseMaxQPS) * threshold)
				targetReplicas := int(math.Ceil(needed))
				if targetReplicas > maxReplicas {
					targetReplicas = maxReplicas
				}
				if targetReplicas < 1 {
					targetReplicas = 1
				}

				// 考慮暖機時間 (由前端傳來的啟動時間清單)
				activeCount := 1
				bootingCount := 0
				if startTimes, ok := comp.Properties["replica_start_times"].([]interface{}); ok {
					for _, tObj := range startTimes {
						startTime := int64(tObj.(float64))
						if elapsedSeconds - startTime >= warmup {
							activeCount++
						} else {
							bootingCount++
						}
					}
				}
				
				// 確保至少有一台基礎機器 (如果是 ASG 通常至少 1 台)
				if activeCount < 1 { activeCount = 1 }

				currentMaxQPS = baseMaxQPS * int64(activeCount)
				compReplicas[id] = activeCount + bootingCount
			} else {
				compReplicas[id] = 1
			}
		}
		compEffectiveMaxQPS[id] = currentMaxQPS

		// 判斷崩潰
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

		// 崩潰閾值設定
		crashThreshold := 1.5
		if comp.Type == component.AutoScalingGroup {
			crashThreshold = 3.0 // ASG 具有一定的彈性緩衝，允許短暫過載以等待機器啟動
		} else if comp.Type == component.MessageQueue || comp.Type == component.ObjectStorage {
			crashThreshold = 50.0 // MQ 和 ObjectStorage 非常難以崩潰
		} else if comp.Type == component.LoadBalancer || comp.Type == component.CDN || comp.Type == component.WAF {
			crashThreshold = 5.0 // Infra 組件相對耐用
		}

		if !isGracePeriod && currentMaxQPS > 0 && potentialTotalLoad > int64(float64(currentMaxQPS)*crashThreshold) {
			crashedNodes[id] = true
			return // 崩潰，流量在此斷掉
		}

		visited[id] = true
		
		// 截斷邏輯 (Capping)
		// 這裡我們當前收到的是 input。
		// 我們知道這個節點總共收到了 potentialTotalLoad。
		// 如果 potentialTotalLoad > maxQPS，則每個來源的流量都應該被等比例縮減 (Throttling)
		// 縮減比例 factor = maxQPS / potentialTotalLoad
		// 實際處理流量 = input * factor
		
		actualProcessed := input
		if currentMaxQPS > 0 && potentialTotalLoad > currentMaxQPS {
			factor := float64(currentMaxQPS) / float64(potentialTotalLoad)
			actualProcessed = int64(float64(input) * factor)
		}
		
		// componentProcessedQPS[id] += actualProcessed // 如果需要統計實際處理量

		outputQPS := actualProcessed
		if comp.Type == component.Cache || comp.Type == component.CDN {
			outputQPS = int64(float64(actualProcessed) * 0.2)
		}

		// 計算「成功取得資料」的量 (Data Fulfillment)
		fulfilled := int64(0)
		if comp.Type == component.Cache || comp.Type == component.CDN {
			// 快取命中的部分算成功
			fulfilled = actualProcessed - outputQPS
		} else if comp.Type == component.Database || comp.Type == component.ObjectStorage || comp.Type == component.SearchEngine {
			// 直接抵達資料庫或儲存層算成功
			fulfilled = actualProcessed
		}
		totalFulfilledQPS += fulfilled

		downstreamCount := int64(len(adj[id]))
		if downstreamCount > 0 {
			splitQPS := outputQPS / downstreamCount
			for _, nextID := range adj[id] {
				finalSplit := splitQPS

				// Message Queue PULL Mode: 限制輸出量到下游的最大容量，防止主動壓垮下游
				if comp.Type == component.MessageQueue {
					if mode, ok := comp.Properties["delivery_mode"].(string); ok && mode == "PULL" {
						downstreamComp, exists := compMap[nextID]
						if exists {
							dsMaxCap := getMaxPotentialCapacity(downstreamComp)
							if dsMaxCap > 0 && finalSplit > dsMaxCap {
								finalSplit = dsMaxCap
							}
						}
					}
				}

				propagateFlow(nextID, finalSplit, newPathVisited)
			}
		}
	}

	for _, root := range roots {
		// Traffic Source 不受 MaxQPS 限制，也不會崩潰
		// 直接傳遞完整流量
		// 但我們還是要呼叫 propagateFlow 來觸發下游
		// 這裡我們直接模擬分流傳給下游，避免 TrafficSource 本身被判定崩潰
		// 或者簡單點：TrafficSource 的 maxQPS 是無限大
		compLoads[root] = currentQPS
		visited[root] = true
		
		downstreamCount := int64(len(adj[root]))
		if downstreamCount > 0 {
			split := currentQPS / downstreamCount
			for _, nextID := range adj[root] {
				propagateFlow(nextID, split, make(map[string]bool))
			}
		}
	}

	// 5. 綜合評估 (以資料獲取成功率為核心)
	successRate := 0.0
	if currentQPS > 0 {
		successRate = float64(totalFulfilledQPS) / float64(currentQPS)
		if successRate > 1.0 {
			successRate = 1.0
		}
	}

	// 綜合可靠性維度 (考慮冗餘設計)
	reliabilityScore := 100.0 - (float64(len(crashedNodes)) * 10.0)
	for _, comp := range compMap {
		if comp.Type == component.Database {
			if mode, ok := comp.Properties["replication_mode"].(string); ok && mode == "MASTER_SLAVE" {
				if v, ok := comp.Properties["slave_count"].(float64); ok && v > 0 {
					reliabilityScore += 10.0 // 有 MS 備援加分
				}
			}
		}
	}
	if reliabilityScore > 100 { reliabilityScore = 100 }
	if reliabilityScore < 0 { reliabilityScore = 0 }

	totalScore := (successRate * 90.0) + (reliabilityScore * 0.1)

	// 延遲模擬
	congestionFactor := 1.0
	for id, load := range compLoads {
		maxQPS := compEffectiveMaxQPS[id]
		if maxQPS == 0 {
			maxQPS = getCompMaxQPS(compMap[id])
		}
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

	comment := "系統穩定運行中。"
	if successRate < 0.1 {
		comment = "警告：使用者幾乎拿不到資料，請檢查伺服器與資料庫連線！"
	} else if successRate < 0.95 {
		comment = "提示：部分請求失敗，建議優化架構或增加容量。"
	}

	scores := []evaluation.Score{
		{Dimension: "System Health", Value: totalScore, Comment: comment},
		{Dimension: "Performance", Value: math.Max(0, 100-(avgLatency-50)/10), Comment: fmt.Sprintf("平均延遲: %.1f ms", avgLatency)},
		{Dimension: "Reliability", Value: reliabilityScore, Comment: "基於冗餘設計與崩潰頻率的可靠性評分。"},
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
		ScenarioID:          d.ScenarioID,
		TotalScore:          totalScore,
		Scores:              scores,
		Passed:              totalScore >= 95.0,
		AvgLatencyMS:        avgLatency,
		TotalQPS:            currentQPS,
		CreatedAt:           time.Now().Unix(),
		ActiveComponentIDs:  activeIDs,
		CrashedComponentIDs: crashedIDs,
		ComponentLoads:           compLoads,
		ComponentEffectiveMaxQPS: compEffectiveMaxQPS,
		IsBurstActive:           isBurstActive,
		ComponentReplicas:       compReplicas,
		RetentionRate:           retentionRate,
		IsRandomDrop:            isRandomDrop,
		FulfilledQPS:            totalFulfilledQPS,
	}, nil
}

func getCompMaxQPS(comp component.Component) int64 {
	base := int64(0)
	if v, ok := comp.Properties["max_qps"].(int64); ok {
		base = v
	} else if v, ok := comp.Properties["max_qps"].(float64); ok {
		base = int64(v)
	}

	if comp.Type == component.Database {
		if mode, ok := comp.Properties["replication_mode"].(string); ok && mode == "MASTER_SLAVE" {
			slaves := 0
			if v, ok := comp.Properties["slave_count"].(float64); ok {
				slaves = int(v)
			}
			// 主從架構：Slaves 增加讀取 QPS 能力 (假設提升 100% per slave)
			return base * int64(1+slaves)
		}
	}
	return base
}

func getMaxPotentialCapacity(comp component.Component) int64 {
	base := getCompMaxQPS(comp)
	if comp.Type == component.WebServer || comp.Type == component.AutoScalingGroup {
		if auto, ok := comp.Properties["auto_scaling"].(bool); ok && auto {
			maxReplicas := 5
			if v, ok := comp.Properties["max_replicas"].(float64); ok {
				maxReplicas = int(v)
			} else if v, ok := comp.Properties["max_replicas"].(int64); ok {
				maxReplicas = int(v)
			}
			return base * int64(maxReplicas)
		}
	}
	return base
}

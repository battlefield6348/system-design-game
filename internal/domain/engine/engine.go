package engine

import (
	"fmt"
	"math"
	"system-design-game/internal/domain/component"
	"system-design-game/internal/domain/design"
	"system-design-game/internal/domain/evaluation"
	"system-design-game/internal/domain/scenario"
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
	
	// 生成惡意流量 (Malicious QPS) - 改為突發事件模型 (DDOS 攻擊)
	isAttackActive := false
	currentMaliciousQPS := int64(0)
	
	enableAttacks := false
	for _, root := range roots {
		if v, ok := compMap[root].Properties["enable_attacks"].(bool); ok {
			enableAttacks = v
		}
	}

	// 每 40 秒發動一次持續 5 秒的大型突發攻擊
	if enableAttacks && elapsedSeconds > 15 && elapsedSeconds%40 < 5 {
		isAttackActive = true
		// 攻擊流量強度：基礎 3000 QPS + 隨機波動
		attackIntensity := 3000.0 + math.Abs(math.Sin(float64(elapsedSeconds)))*5000.0
		currentMaliciousQPS = int64(attackIntensity)
	}

	// 4. 核心物理流量模擬：計算負載與截斷
	visited := make(map[string]bool)
	crashedNodes := make(map[string]bool)
	compLoads := make(map[string]int64)             // 紀錄組件收到的「總輸入流量」
	compMaliciousLoads := make(map[string]int64)    // 紀錄組件收到的「惡意請求量」
	compEffectiveMaxQPS := make(map[string]int64)   // 紀錄組件當前的「有效最大處理能力」(含 Auto Scaling)
	compBacklogs := make(map[string]int64)          // 紀錄 MQ 等組件的積壓量
	compCPUUsage := make(map[string]float64)       // 紀錄組件 CPU 使用率
	compRAMUsage := make(map[string]float64)       // 紀錄組件 RAM 使用率

	compReplicas := make(map[string]int)
	for _, c := range d.Components {
		compReplicas[c.ID] = 1
	}

	// 讀寫分離比例 (預設 80% 讀)
	readRatio := 0.8
	for _, root := range roots {
		if v, ok := compMap[root].Properties["read_ratio"].(float64); ok {
			readRatio = v / 100.0
		}
	}
	currentReadQPS := int64(float64(currentQPS) * readRatio)
	currentWriteQPS := currentQPS - currentReadQPS

	var totalFulfilledQPS int64
	var totalReadFulfilled int64
	var totalWriteFulfilled int64
	var totalOperationalCost float64
	var totalBaseLatency float64
	var consistencyScore = 100.0
	var securityIncidents float64 // 紀錄抵達敏感節點的惡意流量

	// Pass 1: 計算潛在總負載 (Potential Load)
	// 這一步只累加流量，不進行截斷，也不觸發崩潰邏輯
	// 目的：讓每個節點知道自己「將會」收到多少流量
	passesInputLoad := make(map[string]int64)
	
	var calculateLoad func(string, int64, int64, int64, map[string]bool)
	calculateLoad = func(id string, read, write, mal int64, pathVisited map[string]bool) {
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

		passesInputLoad[id] += (read + write + mal)

		downstreamCount := int64(len(adj[id]))
		if downstreamCount > 0 {
			comp := compMap[id]
			outRead, outWrite := read, write
			
			// 快取/CDN 特性：讀取會被攔截 (Hit)，寫入會穿透 (Pass-through)
			if comp.Type == component.Cache || comp.Type == component.CDN {
				outRead = int64(float64(read) * 0.2) // 假設 80% Cache Hit
				outWrite = write                      // 寫入 100% 穿透
			}
			
			malOutput := int64(float64(mal) * 1.0)
			if comp.Type == component.WAF {
				malOutput = int64(float64(mal) * 0.1)
			}
			
			splitRead := outRead / downstreamCount
			splitWrite := outWrite / downstreamCount
			malSplit := malOutput / downstreamCount
			for _, nextID := range adj[id] {
				calculateLoad(nextID, splitRead, splitWrite, malSplit, newPathVisited)
			}
		}
	}

	for _, root := range roots {
		calculateLoad(root, currentReadQPS, currentWriteQPS, currentMaliciousQPS, make(map[string]bool))
	}


	// Pass 2: 實際流量傳播 (Actual Flow Propagation)
	var propagateFlow func(string, int64, int64, int64, map[string]bool)
	propagateFlow = func(id string, read, write, mal int64, pathVisited map[string]bool) {
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

		// 基礎開課成本 (Setup + Operational)
		compCost := comp.OperationalCost
		if compCost == 0 {
			// 預設維運成本
			switch comp.Type {
			case component.WAF: compCost = 0.15
			case component.LoadBalancer: compCost = 0.1
			case component.Database: compCost = 0.5
			case component.Cache: compCost = 0.3
			case component.WebServer: compCost = 0.2
			case component.APIGateway: compCost = 0.15
			case component.NoSQL: compCost = 0.4
			}
		}
		totalOperationalCost += compCost

		// 基礎延遲累積
		if v, ok := comp.Properties["base_latency"].(float64); ok {
			totalBaseLatency += v
		} else if v, ok := comp.Properties["base_latency"].(int64); ok {
			totalBaseLatency += float64(v)
		} else {
			// 預設延遲
			switch comp.Type {
			case component.LoadBalancer:
				totalBaseLatency += 5.0
			case component.WebServer:
				totalBaseLatency += 20.0
			case component.Database:
				totalBaseLatency += 50.0
			case component.MessageQueue:
				totalBaseLatency += 200.0 // MQ 的非同步延遲代價
				consistencyScore -= 5.0    // MQ 引入最終一致性風險
			case component.Cache, component.CDN:
				totalBaseLatency += 2.0
				consistencyScore -= 10.0 // Cache 引入資料不一致風險
			case component.APIGateway:
				totalBaseLatency += 2.0
			case component.NoSQL:
				totalBaseLatency += 10.0
				consistencyScore -= 15.0 // NoSQL 通常為最終一致性
			}
		}

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
				// ASG 額外成本：每台機器都要算錢
				totalOperationalCost += comp.OperationalCost * float64(activeCount+bootingCount-1)
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
		
		// --- 資源消耗計算 ---
		// CPU 消耗：與負載成正比，100% 負荷代表達到 MaxQPS
		cpu := 10.0 // 基礎 CPU 消耗 (Idle)
		if currentMaxQPS > 0 {
			cpu += (float64(potentialTotalLoad) / float64(currentMaxQPS)) * 90.0
		}
		compCPUUsage[id] = math.Min(150.0, cpu) // 最高顯示到 150% (代表嚴重過載)

		// RAM 消耗：不同組件有不同特性
		ram := 15.0 // 基礎 RAM 消耗
		switch comp.Type {
		case component.Cache:
			// 快取組件：RAM 隨流量增長而填滿 (模擬快取物件增加)
			ram += (float64(potentialTotalLoad) / float64(currentMaxQPS)) * 70.0
		case component.MessageQueue:
			// MQ 組件：RAM 隨積壓量 (Backlog) 增加
			prevBacklog := int64(0)
			if v, ok := comp.Properties["backlog"].(float64); ok { prevBacklog = int64(v) }
			ram += (float64(prevBacklog) / 50000.0) * 80.0 // 假設 5萬筆積壓會爆 RAM
		case component.Database, component.NoSQL:
			ram += 30.0 + (float64(potentialTotalLoad) / 10000.0) * 20.0
		default:
			ram += (float64(potentialTotalLoad) / 20000.0) * 10.0
		}
		compRAMUsage[id] = math.Min(120.0, ram)

		// OOM (Out of Memory) 判定
		if !isGracePeriod && ram > 100.0 {
			crashedNodes[id] = true
			return // OOM 崩潰
		}
		// ------------------

		visited[id] = true
		compMaliciousLoads[id] += mal
		
		// 截斷邏輯 (Capping)
		actualRead := read
		actualWrite := write
		actualMalProcessed := mal

		// WAF 過濾
		if comp.Type == component.WAF {
			actualMalProcessed = int64(float64(mal) * 0.1)
			actualRead = int64(float64(read) * 0.98)
			actualWrite = int64(float64(write) * 0.98)
		}
		
		// 資源放大效應：寫入操作通常比讀取消耗多 3-5 倍 CPU
		effectiveResourceLoad := float64(actualRead) + float64(actualWrite)*4.0
		
		// 重新計算 CPU (考慮寫入加權)
		if currentMaxQPS > 0 {
			compCPUUsage[id] = math.Min(150.0, 10.0+(effectiveResourceLoad/float64(currentMaxQPS))*90.0)
		}

		// 安全判定
		if (comp.Type == component.Database || comp.Type == component.NoSQL) && actualMalProcessed > 0 {
			isProtected := false
			for vid := range visited {
				if compMap[vid].Type == component.APIGateway {
					isProtected = true
					break
				}
			}
			threat := float64(actualMalProcessed)
			if isProtected { threat *= 0.5 }
			securityIncidents += threat
		}

		var queuingDelay float64
		var actualProcessed int64

		// Message Queue 特殊邏輯... (略)
		if comp.Type == component.MessageQueue {
			// 1. MQ 自身的 I/O 吞吐量限制
			mqIoLimit := currentMaxQPS 
			
			// 2. 下游消費者的總處理能力 (Consumer-Driven)
			downstreamCapacity := int64(0)
			for _, nextID := range adj[id] {
				if dsComp, ok := compMap[nextID]; ok {
					// 如果下游已經掛了，則不提供處理能力
					if crashedNodes[nextID] {
						continue
					}
					// 這裡拿的是下游的基礎或有效容量
					// 簡化模型：拿下游的 getCompMaxQPS (如果是 WebServer，通常是單機或 Cluster 的容量)
					downstreamCapacity += getCompMaxQPS(dsComp)
				}
			}

			// 實際處理速度受限於「MQ 吞吐量」與「消費者能力」的最小值
			effectiveProcessingRate := mqIoLimit
			if downstreamCapacity > 0 && downstreamCapacity < mqIoLimit {
				effectiveProcessingRate = downstreamCapacity
			} else if downstreamCapacity == 0 {
				// 如果下游完全沒有可用的消費者，則處理能力為 0
				effectiveProcessingRate = 0
			}

			// 從 Properties 獲取上一次的積壓量
			prevBacklog := int64(0)
			if v, ok := comp.Properties["backlog"].(float64); ok {
				prevBacklog = int64(v)
			} else if v, ok := comp.Properties["backlog"].(int64); ok {
				prevBacklog = v
			}

			// 嘗試處理
			attemptLoad := read + write + prevBacklog
			if effectiveProcessingRate > 0 && attemptLoad > effectiveProcessingRate {
				actualProcessed = effectiveProcessingRate
				compBacklogs[id] = attemptLoad - effectiveProcessingRate
			} else {
				actualProcessed = attemptLoad
				compBacklogs[id] = 0
			}
			// MQ 比例縮減讀寫
			ratio := 1.0
			if attemptLoad > 0 { ratio = float64(actualProcessed) / float64(attemptLoad) }
			actualRead = int64(float64(read) * ratio)
			actualWrite = int64(float64(write) * ratio)

			// MQ 延遲代價
			if effectiveProcessingRate > 0 {
				queuingDelay = (float64(compBacklogs[id]) / float64(effectiveProcessingRate)) * 1000.0
			}
			
			// 更新顯示用的有效 MaxQPS
			compEffectiveMaxQPS[id] = effectiveProcessingRate
		} else {
			if currentMaxQPS > 0 && potentialTotalLoad > currentMaxQPS {
				factor := float64(currentMaxQPS) / float64(potentialTotalLoad)
				actualRead = int64(float64(read) * factor)
				actualWrite = int64(float64(write) * factor)
			}
			actualProcessed = actualRead + actualWrite
		}
		totalBaseLatency += queuingDelay
		
		// componentProcessedQPS[id] += actualProcessed // 如果需要統計實際處理量

		outRead, outWrite := actualRead, actualWrite
		if comp.Type == component.Cache || comp.Type == component.CDN {
			// Cache/CDN 僅處理讀取，寫入流量 100% 穿透
			outRead = int64(float64(actualRead) * 0.2) // 假設 80% 命中
			outWrite = actualWrite // 寫入流量直接穿透
		}

		// 計算「成功取得資料」
		fulfilledRead, fulfilledWrite := int64(0), int64(0)
		if comp.Type == component.Cache || comp.Type == component.CDN {
			fulfilledRead = actualRead - outRead
		} else if comp.Type == component.Database || comp.Type == component.NoSQL || comp.Type == component.ObjectStorage || comp.Type == component.SearchEngine {
			// Slave DB 限制：只能處理讀取
			isSlave := false
			if mode, ok := comp.Properties["replication_mode"].(string); ok && mode == "SLAVE" {
				isSlave = true
			}
			
			fulfilledRead = actualRead
			if !isSlave {
				fulfilledWrite = actualWrite
			} else if actualWrite > 0 {
				// 寫到 Slave 會延遲降分或觸發警告 (這裡簡單處理)
				consistencyScore -= 1.0
			}
		}
		totalReadFulfilled += fulfilledRead
		totalWriteFulfilled += fulfilledWrite
		totalFulfilledQPS = totalReadFulfilled + totalWriteFulfilled

		downstreamCount := int64(len(adj[id]))
		if downstreamCount > 0 {
			splitRead := outRead / downstreamCount
			splitWrite := outWrite / downstreamCount
			malSplit := actualMalProcessed / downstreamCount
			for _, nextID := range adj[id] {
				finalReadSplit := splitRead
				finalWriteSplit := splitWrite
				finalMalSplit := malSplit

				if comp.Type == component.MessageQueue {
					if mode, ok := comp.Properties["delivery_mode"].(string); ok && mode == "PULL" {
						downstreamComp, exists := compMap[nextID]
						if exists {
							dsMaxCap := getMaxPotentialCapacity(downstreamComp)
							// MQ PULL 模式下，下游拉取量受限於自身容量
							if dsMaxCap > 0 && (finalReadSplit+finalWriteSplit) > dsMaxCap {
								// 按比例分配讀寫流量
								totalSplit := finalReadSplit + finalWriteSplit
								if totalSplit > 0 {
									ratio := float64(dsMaxCap) / float64(totalSplit)
									finalReadSplit = int64(float64(finalReadSplit) * ratio)
									finalWriteSplit = int64(float64(finalWriteSplit) * ratio)
								}
							}
						}
					}
				}

				propagateFlow(nextID, finalReadSplit, finalWriteSplit, finalMalSplit, newPathVisited)
			}
		}
	}

	for _, root := range roots {
		compLoads[root] = currentQPS + currentMaliciousQPS
		visited[root] = true
		
		downstreamCount := int64(len(adj[root]))
		if downstreamCount > 0 {
			splitRead := currentReadQPS / downstreamCount
			splitWrite := currentWriteQPS / downstreamCount
			malSplit := currentMaliciousQPS / downstreamCount
			for _, nextID := range adj[root] {
				propagateFlow(nextID, splitRead, splitWrite, malSplit, make(map[string]bool))
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

	// 安全評分計算
	securityScore := 100.0 - (securityIncidents * 0.5) // 每有一點惡意流量抵達 DB 扣 0.5 分
	if securityScore < 0 { securityScore = 0 }

	totalScore := (successRate * 70.0) + (reliabilityScore * 0.1) + (securityScore * 0.2)

	// 延遲模擬
	congestionFactor := 1.0
	for _, cpuUsage := range compCPUUsage {
		// CPU 過載導致的延遲：當 CPU > 90% 時發生指數級增長
		if cpuUsage > 90.0 {
			// (cpu - 90) / 10 的平方，讓過載後的延遲瞬間飆升
			factor := 1.0 + math.Pow((cpuUsage-90.0)/10.0, 3.0)
			if factor > congestionFactor {
				congestionFactor = factor
			}
		}
	}
	
	// 原有的排隊延遲邏輯 (基於 QPS/MaxQPS)
	for id, load := range compLoads {
		maxQPS := compEffectiveMaxQPS[id]
		if maxQPS == 0 {
			maxQPS = getCompMaxQPS(compMap[id])
		}
		if maxQPS > 0 {
			utilization := float64(load) / float64(maxQPS)
			if utilization > 0.95 { // 接近極限時增加額外延遲
				factor := 1.0 + (utilization-0.95)*10.0
				if factor > congestionFactor {
					congestionFactor = factor
				}
			}
		}
	}
	avgLatency := totalBaseLatency * congestionFactor
	if avgLatency > 5000 {
		avgLatency = 5000
	}

	// 成本評估：從關卡讀取預算限制
	budget := 50.0
	for _, c := range s.Constraints {
		if c.Type == "budget" {
			budget = float64(c.Value)
		}
	}

	costScore := 100.0
	if totalOperationalCost > budget {
		// 超過預算每一塊錢扣 5 分 (更嚴厲的預算懲罰)
		costScore = math.Max(0, 100.0-(totalOperationalCost-budget)*5)
	}

	if consistencyScore < 0 { consistencyScore = 0 }

	comment := "系統穩定運行中。"
	if successRate < 0.1 {
		comment = "警告：使用者幾乎拿不到資料，請檢查伺服器與資料庫連線！"
	} else if successRate < 0.95 {
		comment = "提示：部分請求失敗，建議優化架構或增加容量。"
	}

	scores := []evaluation.Score{
		{Dimension: "System Health", Value: totalScore, Comment: comment},
		{Dimension: "Performance", Value: math.Max(0, 100-(avgLatency-100)/20), Comment: fmt.Sprintf("平均延遲: %.1f ms", avgLatency)},
		{Dimension: "Reliability", Value: reliabilityScore, Comment: "基於冗餘設計與崩潰頻率的可靠性評分。"},
		{Dimension: "Security", Value: securityScore, Comment: "抵達核心節點的惡意流量會降低安全性。"},
		{Dimension: "Cost Efficiency", Value: costScore, Comment: fmt.Sprintf("每秒運維成本: $%.2f", totalOperationalCost)},
		{Dimension: "Data Consistency", Value: consistencyScore, Comment: "快取或異步隊列會降低即時一致性。"},
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
		TotalReadQPS:             currentReadQPS,
		TotalWriteQPS:            currentWriteQPS,
		CreatedAt:           elapsedSeconds,
		ActiveComponentIDs:  activeIDs,
		CrashedComponentIDs: crashedIDs,
		ComponentLoads:           compLoads,
		ComponentEffectiveMaxQPS: compEffectiveMaxQPS,
		IsBurstActive:           isBurstActive,
		IsAttackActive:          isAttackActive,
		ComponentReplicas:       compReplicas,
		RetentionRate:           retentionRate,
		IsRandomDrop:            isRandomDrop,
		TotalQPS:                 currentQPS,
		FulfilledQPS:            totalFulfilledQPS,
		CostPerSec:              totalOperationalCost,
		ComponentBacklogs:       compBacklogs,
		SecurityScore:           securityScore,
		ComponentMaliciousLoads: compMaliciousLoads,
		ComponentCPUUsage:       compCPUUsage,
		ComponentRAMUsage:       compRAMUsage,
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
	} else if comp.Type == component.NoSQL || comp.Type == component.APIGateway {
		return base
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

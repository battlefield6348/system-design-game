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

	// 3. 遍歷圖形，找出有效的 WebServer
	visited := make(map[string]bool)
	reachableWebServers := make(map[string]bool)
	
	var traverse func(string)
	traverse = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		
		comp := compMap[id]
		if comp.Type == component.WebServer {
			reachableWebServers[id] = true
		}
		
		for _, nextID := range adj[id] {
			traverse(nextID)
		}
	}

	for _, root := range roots {
		traverse(root)
	}

	// 4. 計算有效容量
	var totalCapacity int64
	for id := range reachableWebServers {
		comp := compMap[id]
		if maxQPS, ok := comp.Properties["max_qps"].(int64); ok {
			totalCapacity += maxQPS
		} else if maxQPSFloat, ok := comp.Properties["max_qps"].(float64); ok {
			totalCapacity += int64(maxQPSFloat)
		}
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

	// 如果超過所有階段，維持在最後一個階段的 EndQPS
	if currentQPS == 0 && len(s.Phases) > 0 {
		currentQPS = s.Phases[len(s.Phases)-1].EndQPS
	}

	// 計算錯誤率
	errorRate := 0.0
	if currentQPS > totalCapacity && totalCapacity > 0 {
		errorRate = float64(currentQPS-totalCapacity) / float64(currentQPS)
	} else if (totalCapacity == 0 || len(roots) == 0) && currentQPS > 0 {
		errorRate = 1.0
	}

	health := (1.0 - errorRate) * 100.0

	scores := []evaluation.Score{
		{Dimension: "Topology", Value: float64(len(reachableWebServers)) * 10, Comment: fmt.Sprintf("有效連接伺服器: %d 台", len(reachableWebServers))},
		{Dimension: "Capacity", Value: health, Comment: fmt.Sprintf("有效總容量: %d QPS, 當前流量: %d QPS", totalCapacity, currentQPS)},
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
	}, nil
}

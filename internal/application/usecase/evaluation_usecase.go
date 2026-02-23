package usecase

import (
	"system-design-game/internal/domain/engine"
	"system-design-game/internal/domain/evaluation"
)

// EvaluationUseCase 處理系統設計評估的業務流程
type EvaluationUseCase struct {
	engine engine.Engine
}

func NewEvaluationUseCase(e engine.Engine) *EvaluationUseCase {
	return &EvaluationUseCase{engine: e}
}

// Evaluate 執行評估邏輯
func (uc *EvaluationUseCase) Evaluate(designID string, elapsedSeconds int64) (*evaluation.Result, error) {
	// 這裡可以加入一些應用層的邏輯，例如紀錄評估次數或暫存結果
	return uc.engine.Evaluate(designID, elapsedSeconds)
}

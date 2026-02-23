package usecase

import (
	"system-design-game/internal/domain/scenario"
)

// ScenarioUseCase 處理與關卡情境相關的業務流程
type ScenarioUseCase struct {
	repo scenario.Repository
}

func NewScenarioUseCase(repo scenario.Repository) *ScenarioUseCase {
	return &ScenarioUseCase{repo: repo}
}

// GetScenario 獲取指定的關卡資訊
func (uc *ScenarioUseCase) GetScenario(id string) (*scenario.Scenario, error) {
	return uc.repo.GetByID(id)
}

// ListScenarios 列出所有可用的關卡
func (uc *ScenarioUseCase) ListScenarios() ([]*scenario.Scenario, error) {
	return uc.repo.ListAll()
}

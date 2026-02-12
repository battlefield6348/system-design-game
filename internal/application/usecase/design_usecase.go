package usecase

import (
	"system-design-game/internal/domain/design"
)

// DesignUseCase 處理與設計圖相關的業務流程
type DesignUseCase struct {
	repo design.Repository
}

func NewDesignUseCase(repo design.Repository) *DesignUseCase {
	return &DesignUseCase{repo: repo}
}

// SaveDesign 儲存玩家的設計
func (uc *DesignUseCase) SaveDesign(d *design.Design) error {
	// TODO: 可以在這裡加入驗證邏輯，例如檢查組件連線是否合法
	return uc.repo.Save(d)
}

// GetDesign 獲取指定的設計圖
func (uc *DesignUseCase) GetDesign(id string) (*design.Design, error) {
	return uc.repo.GetByID(id)
}

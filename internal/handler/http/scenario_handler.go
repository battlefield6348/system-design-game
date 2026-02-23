package http

import (
	"net/http"
	"system-design-game/internal/application/usecase"

	"github.com/gin-gonic/gin"
)

// ScenarioHandler 處理關卡相關的 HTTP 請求
type ScenarioHandler struct {
	scenarioUC *usecase.ScenarioUseCase
}

// NewScenarioHandler 建立新的 ScenarioHandler
func NewScenarioHandler(suc *usecase.ScenarioUseCase) *ScenarioHandler {
	return &ScenarioHandler{
		scenarioUC: suc,
	}
}

// List Scenarios 列出所有可用關卡
func (h *ScenarioHandler) List(c *gin.Context) {
	scenarios, err := h.scenarioUC.ListScenarios()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scenarios)
}

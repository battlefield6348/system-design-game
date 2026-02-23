package http

import (
	"net/http"
	"system-design-game/internal/application/usecase"
	"system-design-game/internal/domain/design"

	"github.com/gin-gonic/gin"
)

// DesignHandler 處理設計圖相關的 HTTP 請求
type DesignHandler struct {
	designUC *usecase.DesignUseCase
	evalUC   *usecase.EvaluationUseCase
}

// NewDesignHandler 建立新的 DesignHandler
func NewDesignHandler(duc *usecase.DesignUseCase, euc *usecase.EvaluationUseCase) *DesignHandler {
	return &DesignHandler{
		designUC: duc,
		evalUC:   euc,
	}
}

// Evaluate 評估設計圖
func (h *DesignHandler) Evaluate(c *gin.Context) {
	id := c.Param("design_id")
	// 這裡可以傳入時間軸參數，目前先預設為 0
	res, err := h.evalUC.Evaluate(id, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// Save 儲存設計圖 (Server 範例)
func (h *DesignHandler) Save(c *gin.Context) {
	var d design.Design
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "無效的設計圖格式"})
		return
	}

	if err := h.designUC.SaveDesign(&d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "設計圖儲存成功 (Server)", "id": d.ID})
}

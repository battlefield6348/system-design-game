package main

import (
	"log"
	"net/http"
	"system-design-game/internal/domain/engine"
	"system-design-game/internal/infrastructure/persistence"

	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化基礎組件 (單人模式優先)
	designRepo := persistence.NewInMemDesignRepository()
	scenarioRepo := persistence.NewInMemScenarioRepository()
	evalEngine := engine.NewSimpleEngine(designRepo, scenarioRepo)

	r := gin.Default()

	// 基礎健康檢查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"message": "系統設計遊戲後端服務已啟動 (單人模式)",
		})
	})

	// 註冊處理程序 (Handlers)
	r.POST("/evaluate/:design_id", func(c *gin.Context) {
		id := c.Param("design_id")
		res, err := evalEngine.Evaluate(id, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	})

	log.Println("伺服器運行在 :8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("無法啟動伺服器: %v", err)
	}
}

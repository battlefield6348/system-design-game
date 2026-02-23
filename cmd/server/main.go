package main

import (
	"log"
	"net/http"
	"system-design-game/internal/application/usecase"
	"system-design-game/internal/domain/engine"
	apphttp "system-design-game/internal/handler/http"
	"system-design-game/internal/infrastructure/persistence"

	"github.com/gin-gonic/gin"
)

func main() {
	// 基礎設施層 (Infrastructure Layer)
	designRepo := persistence.NewInMemDesignRepository()
	scenarioRepo := persistence.NewInMemScenarioRepository()

	// 領域層 (Domain Layer) - 領域服務
	evalEngine := engine.NewSimpleEngine(designRepo, scenarioRepo)

	// 應用層 (Application Layer) - 用例 (Use Cases)
	designUC := usecase.NewDesignUseCase(designRepo)
	scenarioUC := usecase.NewScenarioUseCase(scenarioRepo)
	evalUC := usecase.NewEvaluationUseCase(evalEngine)

	// 介面層 (Interfaces / Presenters) - Handlers
	designHandler := apphttp.NewDesignHandler(designUC, evalUC)
	scenarioHandler := apphttp.NewScenarioHandler(scenarioUC)

	r := gin.Default()

	// 基礎健康檢查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "系統設計遊戲後端服務已啟動 (單人模式)",
		})
	})

	// 註冊處理程序 (Handlers)
	r.POST("/evaluate/:design_id", designHandler.Evaluate)
	r.GET("/scenarios", scenarioHandler.List)
	r.POST("/design", designHandler.Save)

	log.Println("伺服器運行在 :8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("無法啟動伺服器: %v", err)
	}
}

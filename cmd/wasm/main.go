//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"system-design-game/internal/application/usecase"
	"system-design-game/internal/domain/design"
	"system-design-game/internal/domain/engine"
	"system-design-game/internal/infrastructure/persistence"
)

var (
	designUC   *usecase.DesignUseCase
	scenarioUC *usecase.ScenarioUseCase
	evalUC     *usecase.EvaluationUseCase
)

func main() {
	// 基礎設施層
	designRepo := persistence.NewInMemDesignRepository()
	scenarioRepo := persistence.NewInMemScenarioRepository()

	// 領域層
	evalEngine := engine.NewSimpleEngine(designRepo, scenarioRepo)

	// 應用層
	designUC = usecase.NewDesignUseCase(designRepo)
	scenarioUC = usecase.NewScenarioUseCase(scenarioRepo)
	evalUC = usecase.NewEvaluationUseCase(evalEngine)

	// 暴露函數給 JavaScript
	js.Global().Set("goEvaluate", js.FuncOf(evaluate))
	js.Global().Set("goSaveDesign", js.FuncOf(saveDesign))
	js.Global().Set("goListScenarios", js.FuncOf(listScenarios))

	fmt.Println("Wasm 模組已載入 (Clean Architecture 模式)")

	// 保持運行
	select {}
}

func evaluate(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "需要 Design ID"
	}

	designID := args[0].String()
	elapsed := int64(0)
	if len(args) > 1 {
		elapsed = int64(args[1].Int())
	}

	// 透過 UseCase 進行評估
	res, err := evalUC.Evaluate(designID, elapsed)
	if err != nil {
		fmt.Printf("評估失敗: %v\n", err)
		return err.Error()
	}

	jsonRes, _ := json.Marshal(res)
	return string(jsonRes)
}

func saveDesign(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "需要 Design JSON"
	}

	jsonStr := args[0].String()
	var d design.Design
	err := json.Unmarshal([]byte(jsonStr), &d)
	if err != nil {
		return "解析 JSON 失敗: " + err.Error()
	}

	// 透過 UseCase 儲存設計
	err = designUC.SaveDesign(&d)
	if err != nil {
		return "儲存失敗: " + err.Error()
	}

	return nil
}

func listScenarios(this js.Value, args []js.Value) interface{} {
	// 透過 UseCase 取得關卡列表
	scenarios, err := scenarioUC.ListScenarios()
	if err != nil {
		return "取得關卡失敗: " + err.Error()
	}

	jsonRes, _ := json.Marshal(scenarios)
	return string(jsonRes)
}

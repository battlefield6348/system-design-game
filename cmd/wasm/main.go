//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"system-design-game/internal/domain/design"
	"system-design-game/internal/domain/engine"
	"system-design-game/internal/infrastructure/persistence"
)

var (
	designRepo   *persistence.InMemDesignRepository
	scenarioRepo *persistence.InMemScenarioRepository
	evalEngine   *engine.SimpleEngine
)

func main() {
	// 初始化內部邏輯
	designRepo = persistence.NewInMemDesignRepository()
	scenarioRepo = persistence.NewInMemScenarioRepository()
	evalEngine = engine.NewSimpleEngine(designRepo, scenarioRepo)

	// 暴露函數給 JavaScript
	js.Global().Set("goEvaluate", js.FuncOf(evaluate))
	js.Global().Set("goSaveDesign", js.FuncOf(saveDesign))

	fmt.Println("Wasm 模組已載入")

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
	res, err := evalEngine.Evaluate(designID, elapsed)
	if err != nil {
		fmt.Printf("評估失敗: %v\n", err)
		return err.Error()
	}

	// 轉換為 JSON 字串回傳給 JS
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

	err = designRepo.Save(&d)
	if err != nil {
		return "儲存失敗: " + err.Error()
	}

	return nil
}

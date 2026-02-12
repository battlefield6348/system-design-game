//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
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

	fmt.Println("Wasm 模組已載入")

	// 保持運行
	select {}
}

func evaluate(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "需要 Design ID"
	}

	designID := args[0].String()
	res, err := evalEngine.Evaluate(designID)
	if err != nil {
		return err.Error()
	}

	// 轉換為 JSON 字串回傳給 JS
	jsonRes, _ := json.Marshal(res)
	return string(jsonRes)
}

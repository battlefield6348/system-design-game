# 系統設計遊戲 (System Design Game) 架構文件

這個專案旨在建立一個讓玩家練習系統設計的遊戲化平台。玩家可以放置組件（如 Load Balancer, DB, Cache），並由系統評估其設計的優劣。

## 核心架構

我們採用 **微服務架構 (Clean Architecture)** 的概念，確保業務邏輯與外部框架解耦：

- `cmd/server/`: 應用程式入口點。
- `internal/domain/`: 核心業務邏輯（Domain Models, Interfaces, Domain Services）。
- `internal/application/`: 應用層用例 (Use Cases)。
- `internal/infrastructure/`: 基礎設施實現（DB, External API, MQ）。
- `internal/handler/`: 介面層（RESTful API, WebSocket）。

## 核心領域模型 (Pre-alpha)

- **Component (組件)**: 系統設計的最小單位（例如：Redis, PostgreSQL, Nginx）。
- **Design (設計圖)**: 玩家排列組件後的完整拓撲結構。
- **Scenario (情境/關卡)**: 遊戲給出的目標（例如：設計一個每秒 100k 請求的短網址系統）。
- **Evaluation (評估結果)**: 針對設計的評分（可用性、成本、效能）。

## 技術棧 (GitHub Pages 支援)

- **Frontend**: Vite + React + TailwindCSS
- **Core Logic**: Golang (Go 1.25+)
- **Delivery**: **WebAssembly (Wasm)** - 將 Go 後端邏輯編譯為 Wasm 運行於瀏覽器。
- **Database**: LocalStorage / IndexedDB (替代 PostgreSQL)。
- **Real-time**: Local Event Bus (替代 WebSocket)。

## 開發準則

- 所有業務規範應定義在 `domain` 層。
- 註解使用 **繁體中文**。
- 遵循 Go Module 標準。
- **單人優先**：專注於設計評估邏輯 (Evaluation Engine)，確保其能準確反映系統設計品質。

## 部署與開發 (GitHub Pages)

1. **編譯 Wasm**: 執行 `make build-wasm`。
2. **開發模式**: 執行 `make dev-frontend`。
3. **部署**: 執行 `make build-all`，並將 `frontend/dist` 目錄提交至 `gh-pages` 分支。

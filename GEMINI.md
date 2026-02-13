# 系統設計遊戲 (System Design Game) 架構文件

這個專案旨在建立一個讓玩家練習系統設計的遊戲化平台。玩家可以放置組件（如 Load Balancer, DB, Cache），並由系統評估其設計的優劣。

## 核心架構

我們採用 **微服務架構 (Clean Architecture)** 的概念，確保業務邏輯與外部框架解耦：

- `cmd/server/`: 應用程式入口點。
- `internal/domain/`: 核心業務邏輯（Domain Models, Interfaces, Domain Services）。
- `internal/application/`: 應用層用例 (Use Cases)。
- `internal/infrastructure/`: 基礎設施實現（DB, External API, MQ）。
- `internal/handler/`: 介面層（RESTful API, WebSocket）。

## 核心領域模型 (Phase 1)

- **Component (組件)**: 系統設計的最小單位。
  - **運算型**: Web Server, ASG (彈性伸縮組), Worker, **Video Transcoding (影片轉碼)**。
  - **儲存型**: SQL (PostgreSQL), NoSQL (MongoDB), Redis, Object Storage。
  - **流量型**: Load Balancer, API Gateway, CDN, WAF。
- **Evaluation Engine (評估引擎)**:
  - **流量模擬**: 支援 **讀取 (Read)** 與 **寫入 (Write)** 流量分離，並在視覺化連線上以雙線平行顯示。
  - **資源負載**: CPU 與 RAM 消耗模型，上限 100%。RAM 超過 100% 會觸發 **OOM 崩潰**。
  - **Auto Scaling**: ASG 支援基於 **CPU 或 RAM 使用率** 的擴展策略。
  - **容量設計**: 組件 QPS 為固定容量（唯讀），玩家必須透過架構手段（如增加副本、LB 分流）解決瓶頸。

## 技術棧 (GitHub Pages 支援)

- **Frontend**: Vite + React + TailwindCSS + **XYFlow (React Flow)** 高階視覺化。
- **Core Logic**: Golang (Go 1.25+)
- **Delivery**: **WebAssembly (Wasm)** - 將 Go 後端邏輯編譯為 Wasm 運行於瀏覽器。
- **Database**: LocalStorage / IndexedDB (替代 PostgreSQL)。
- **Real-time**: Local Event Bus (替代 WebSocket)。

## 開發準則

- 所有業務規範應定義在 `internal/domain` 層。
- **溝通與註解**：全程使用 **繁體中文**。
- **Git 規範**：Git Commit Message 必須使用 **繁體中文** 撰寫，保持風格統一。
- 遵循 Go Module 標準。
- **單人優先**：專注於設計評估邏輯 (Evaluation Engine)，確保其能準確反映系統設計品質（如影片轉碼的高 CPU 消耗）。

## 部署與開發 (GitHub Pages)

1. **編譯 Wasm**: 執行 `make build-wasm`。
2. **開發模式**: 執行 `make dev-frontend`。
3. **編譯生產環境**: 執行 `make build-all` (產物將輸出至根目錄的 `docs` 資料夾)。
4. **GitHub 部署**: 將 `docs` 資料夾提交至 `main` 分支，並在 GitHub Settings 中將 Pages Source 設定為 `/docs`。

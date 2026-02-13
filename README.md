# 系統設計遊戲 (System Design Game) 功能清單

此文件記錄了「系統設計遊戲」目前已開發的核心功能與邏輯機制，旨在幫助了解當前的系統邊界與玩法。

## 1. 核心組件 (Component System)

遊戲中提供多種系統設計組件，每種組件具有特定的行為與屬性：

| 組件類型 | 代碼 | 優點 | 缺點 / Trade-off | 關鍵屬性 |
| :--- | :--- | :--- | :--- | :--- |
| **流量來源** | `TRAFFIC_SOURCE` | 模擬使用者請求進入點。 | 可能產生突發流量壓垮下游。 | `start_qps`, `burst_traffic` |
| **負載平衡器** | `LOAD_BALANCER` | 分散流量、高穩定性。 | 引入額外的轉發延遲 (約 5ms)。 | `max_qps` |
| **彈性伸縮組** | `AUTO_SCALING_GROUP` | 自動應對流量增長。 | 具有暖機延遲，且多節點維運成本高。 | `max_replicas`, `warmup_seconds` |
| **網頁伺服器** | `WEB_SERVER` | 處理業務邏輯。 | 不同等級伺服器在成本與延遲間需取捨。 | `max_qps`, `base_latency` |
| **資料庫** | `DATABASE` | 資料持久化、高一致性。 | 基礎延遲高且昂貴，擴展性較差。 | `replication_mode`, `slave_count` |
| **快取/CDN** | `CACHE`, `CDN` | 極低延遲、減輕下游負載。 | 可能產生資料不一致 (Stale Data)。 | `max_qps` |
| **訊息隊列** | `MESSAGE_QUEUE` | 緩衝流量、解耦系統。 | 帶來顯著的異步延遲與最終一致性問題。 | `delivery_mode` (PUSH/PULL) |
| **基礎設施** | `WAF`, `S3`, `ES` | 專業分工、極高穩定性。 | 增加架構複雜度與固定維運成本；WAF 可能誤殺 2% 正常流量。 | `max_qps` |

---

## 2. 模擬引擎 (Simulation Engine)

引擎採用 **兩階段物理流量模擬 (Two-Pass Propagation)**，確保負載計算準確：

### A. 負載計算與流動

* **第一階段 (Potential Pass)**：計算所有路徑上「嘗試請求」的總量。用於判斷組件是否過載或該如何進行 Auto Scaling。
* **第二階段 (Actual Pass)**：根據組件的「有效最大處理能力」進行比例截斷 (Throttling)，模擬真實系統的限流行為。

### B. 動態流量模型

* **場景相位 (Phases)**：支援流量隨時間階梯式或平滑增長。
* **隨機波動 (Vibration)**：模擬自然流量的 ±5% 正弦波動。
* **突發流量 (Burst)**：特定時間點出現 5 倍的高壓流量。
* **隨機墜降 (Random Drop)**：模擬不穩定的網路或未知因素導致的流量損失。

### C. 崩潰與復原機制

* **過載崩潰**：當負載超過處理能力的 1.5 倍 (ASG 為 3.0 倍) 時，組件會進入「已崩潰」狀態。
* **保護期 (Grace Period)**：組件剛重啟的 5 秒內不會再次因為過載而崩潰。
* **手動重啟**：玩家可在前端點擊按鈕重啟失效服務。

---

## 3. 評估維度 (Evaluation)

每秒進行一次系統健康度評估：

1. **資料獲取率 (Data Fulfillment)**：核心指標。計算「最終抵達資料持久層或快取命中」的請求佔總請求的比例。
2. **延遲模擬 (Latency)**：根據組件的**利用率 (Utilization)** 計算。當利用率超過 80% 時，延遲會呈指數型增長 (Congestion Factor)。
3. **可靠性評分 (Reliability)**：考慮系統是否有冗餘設計 (如 DB Master-Slave) 以及當前崩潰的節點數量。
4. **留存率 (User Retention)**：如果系統健康度長期低於 95%，使用者將會流失 (-0.5%/sec)；反之則緩慢恢復。

---

## 4. 前端互動功能 (Frontend)

* **視覺化連接器**：基於 React Flow 的拖動佈線系統。
* **即時動態反饋**：
  * **Node 警告**：過載時節點會劇烈震動並變紅。
  * **連線動畫**：連線速度隨流量大小動態化，流量越大，點流動越快。
  * **ASG 可視化**：在 ASG 內部顯示多台 Node 的啟動狀態 (暖機中/運作中) 與分流情況。
* **快捷操作**：
  * `Cmd/Ctrl + C / V`：複製與貼上組件。
  * `Delete / Backspace`：刪除選中的組件或連線。
  * **自動排版 (Auto Layout)**：一鍵使用 Dagre 演算法整理架構圖。
* **Wasm 運行時**：後端邏輯編譯為 WebAssembly 直接在瀏覽器執行，保證模擬的流暢度與私隱。

---

## 5. 技術架構

* **後端語言**：Golang (Clean Architecture)
* **交付方式**：WebAssembly (GopherJS/Wasm)
* **前端框架**：Vite + React + TailwindCSS
* **部署圖**：GitHub Pages (Single Page Application)

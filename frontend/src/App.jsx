import { useState, useEffect } from 'react'
import './App.css'

function App() {
  const [isWasmLoaded, setIsWasmLoaded] = useState(false)
  const [evaluationResult, setEvaluationResult] = useState(null)

  useEffect(() => {
    // 載入 Go Wasm 模組
    const go = new window.Go()
    WebAssembly.instantiateStreaming(fetch("/main.wasm"), go.importObject).then((result) => {
      go.run(result.instance)
      setIsWasmLoaded(true)
    }).catch(err => {
      console.error("無法載入 Wasm:", err)
    })
  }, [])

  const handleEvaluate = () => {
    if (window.goEvaluate) {
      // 呼叫 Go 暴露的函數 (這裡暫時傳入模擬的 ID "s1")
      const resultStr = window.goEvaluate("s1")
      try {
        const result = JSON.parse(resultStr)
        setEvaluationResult(result)
      } catch (e) {
        console.error("解析結果失敗:", e)
      }
    }
  }

  return (
    <div className="container">
      <h1>系統設計遊戲 (Pre-alpha)</h1>
      <p>狀態: {isWasmLoaded ? '✅ Wasm 已就緒' : '⏳ 載入中...'}</p>
      
      <div className="card">
        <button onClick={handleEvaluate} disabled={!isWasmLoaded}>
          開始評估設計 (s1)
        </button>
      </div>

      {evaluationResult && (
        <div className="result">
          <h2>評估結果</h2>
          <p>總分: {evaluationResult.total_score}</p>
          <p>通過: {evaluationResult.passed ? '✅ 是' : '❌ 否'}</p>
          <ul>
            {evaluationResult.scores.map((s, i) => (
              <li key={i}>
                <strong>{s.dimension}</strong>: {s.value} - {s.comment}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

export default App

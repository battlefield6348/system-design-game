import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  addEdge,
  Handle,
  Position,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Server, Activity, Database, Share2, Plus, Play } from 'lucide-react';
import './App.css';

// 自定義節點組件
const CustomNode = ({ data, selected }) => {
  const Icon = data.icon || Server;
  return (
    <div className={`custom-node ${data.type.toLowerCase()} ${selected ? 'selected' : ''}`}>
      <Handle type="target" position={Position.Left} />
      <div className="node-content">
        <Icon size={20} />
        <div className="node-info">
          <div className="node-name">{data.label}</div>
          <div className="node-type">{data.type}</div>
        </div>
      </div>
      <Handle type="source" position={Position.Right} />
    </div>
  );
};

const nodeTypes = {
  custom: CustomNode,
};

function App() {
  const [isWasmLoaded, setIsWasmLoaded] = useState(false);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [evaluationResult, setEvaluationResult] = useState(null);
  const [gameTime, setGameTime] = useState(0);
  const [isAutoEvaluating, setIsAutoEvaluating] = useState(false);

  useEffect(() => {
    const go = new window.Go();
    WebAssembly.instantiateStreaming(fetch("/main.wasm"), go.importObject).then((result) => {
      go.run(result.instance);
      setIsWasmLoaded(true);
      // 初始化一個測試場景
      initDefaultDesign();
    }).catch(err => console.error("Wasm 載入失敗:", err));
  }, []);

  // 自動評估循環
  useEffect(() => {
    let interval;
    if (isWasmLoaded && isAutoEvaluating) {
      interval = setInterval(() => {
        setGameTime(prev => {
          const nextTime = prev + 1;
          handleEvaluate(nextTime);
          return nextTime;
        });
      }, 1000);
    }
    return () => clearInterval(interval);
  }, [isWasmLoaded, isAutoEvaluating, nodes, edges]);

  const initDefaultDesign = () => {
    const initialNodes = [
      {
        id: 'traffic-1',
        type: 'custom',
        position: { x: 50, y: 150 },
        data: { label: '使用者流量', type: '流量來源', icon: Activity },
        deletable: false, // 流量起點不准刪除
      }
    ];
    setNodes(initialNodes);
  };

  const onConnect = useCallback((params) => setEdges((eds) => addEdge(params, eds)), [setEdges]);

  const addComponent = (type, label, icon, properties = {}) => {
    const id = `${type.toLowerCase()}-${Date.now()}`;
    const newNode = {
      id,
      type: 'custom',
      position: { x: 300, y: 150 },
      data: { label, type, icon, properties },
    };
    setNodes((nds) => nds.concat(newNode));
  };

  const handleEvaluate = (currentTime = gameTime) => {
    if (!window.goSaveDesign || !window.goEvaluate) return;

    // 將 React Flow 狀態轉換為 Go 領域模型
    const design = {
      id: "live-design",
      scenario_id: "s1",
      components: nodes.map(n => ({
        id: n.id,
        name: n.data.label,
        type: n.data.type,
        properties: n.data.properties || { max_qps: 1000 }
      })),
      connections: edges.map(e => ({
        from_id: e.source,
        to_id: e.target,
        protocol: "HTTP"
      }))
    };

    // 同步到 Wasm
    window.goSaveDesign(JSON.stringify(design));

    // 執行評估 (傳入當前遊戲秒數)
    const resultStr = window.goEvaluate("live-design", currentTime);
    try {
      setEvaluationResult(JSON.parse(resultStr));
    } catch (e) {
      console.error("解析評估結果失敗:", e);
    }
  };

  const toggleAutoEvaluate = () => {
    setIsAutoEvaluating(!isAutoEvaluating);
  };

  return (
    <div className="game-container">
      <header className="game-header">
        <div className="logo">
          <Share2 color="#6366f1" />
          <h1>系統設計遊戲</h1>
        </div>
        <div className="status-bar">
          <span className={`badge ${isWasmLoaded ? 'success' : 'warning'}`}>
            {isWasmLoaded ? '引擎已連線' : '引擎啟動中...'}
          </span>
          {evaluationResult && (
            <div className="live-metrics">
              <span className="metric">健康度: {evaluationResult.total_score.toFixed(1)}%</span>
              <span className="metric">即時 QPS: {evaluationResult.total_qps}</span>
            </div>
          )}
          <button
            className={`btn-primary ${isAutoEvaluating ? 'active' : 'warning'}`}
            onClick={() => setIsAutoEvaluating(!isAutoEvaluating)}
            disabled={!isWasmLoaded}
          >
            <Play size={16} /> {isAutoEvaluating ? '暫停模擬' : '開啟模擬'}
          </button>
        </div>
      </header>

      <main className="game-main">
        <aside className="tool-panel">
          <h3>基礎設施工具</h3>
          <div className="tool-list">
            <button onClick={() => addComponent('WEB_SERVER', '標準伺服器', Server, { max_qps: 1000 })}>
              <Plus size={14} /> 伺服器 (1k QPS)
            </button>
            <button onClick={() => addComponent('LOAD_BALANCER', '負載平衡器', Share2, { max_qps: 20000 })}>
              <Plus size={14} /> 負載平衡器
            </button>
            <button onClick={() => addComponent('DATABASE', '資料庫', Database, { capacity: 500 })}>
              <Plus size={14} /> 資料庫
            </button>
          </div>

          {evaluationResult && (
            <div className="eval-details">
              <h4>系統診斷報告</h4>
              {evaluationResult.scores.map((s, i) => (
                <div key={i} className="score-item">
                  <span className="dim">{s.dimension}</span>
                  <span className="val">{s.value.toFixed(0)}</span>
                  <p className="comment">{s.comment}</p>
                </div>
              ))}
            </div>
          )}
        </aside>

        <section className="canvas-area">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            nodeTypes={nodeTypes}
            fitView
          >
            <Background color="#333" gap={20} />
            <Controls />
            <MiniMap nodeStrokeWidth={3} zoomable pannable />
          </ReactFlow>
        </section>
      </main>
    </div>
  );
}

export default App;

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
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  useHandleConnections,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Server, Activity, Database, Share2, Plus, Play, X } from 'lucide-react';
import './App.css';

// 自定義連線組件 (帶有刪除按鈕)
const CustomEdge = ({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style = {},
  markerEnd,
  data,
}) => {
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  return (
    <>
      <BaseEdge path={edgePath} markerEnd={markerEnd} style={style} className={data?.animated ? 'animated' : ''} />
      <EdgeLabelRenderer>
        <div
          style={{
            position: 'absolute',
            transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
            fontSize: 12,
            pointerEvents: 'all',
          }}
          className="nodrag nopan"
        >
          <button className="edge-delete-btn" onClick={() => data.onDelete(id)}>
            <X size={10} strokeWidth={4} />
          </button>
        </div>
      </EdgeLabelRenderer>
    </>
  );
};

// 自定義節點組件
const CustomNode = ({ data, selected, id }) => {
  const Icon = data.icon || Server;
  const isTraffic = data.type === 'TRAFFIC_SOURCE';
  const isServer = data.type === 'WEB_SERVER';

  // 監控連線數量
  const connectionsIn = useHandleConnections({ type: 'target', id: 't' });
  const connectionsOut = useHandleConnections({ type: 'source', id: 's' });

  const isTargetLimited = isServer && connectionsIn.length >= 1;
  const isSourceLimited = isTraffic && connectionsOut.length >= 1;

  return (
    <div className={`custom-node ${data.type.toLowerCase()} ${selected ? 'selected' : ''} ${data.active ? 'active' : ''}`}>
      {!isTraffic && (
        <div className="delete-btn" onClick={(e) => {
          e.stopPropagation();
          data.onDelete(id);
        }}>
          <X size={12} strokeWidth={3} />
        </div>
      )}
      <Handle
        id="t"
        type="target"
        position={Position.Left}
        isConnectable={!isTargetLimited}
        className={isTargetLimited ? 'handle-limited' : ''}
      />
      <div className="node-content">
        <Icon size={20} />
        <div className="node-info">
          <div className="node-name">{data.label}</div>
          <div className="node-type">{data.type}</div>
          {data.load !== undefined && (
            <div className={`node-stats ${isSourceLimited ? 'limited' : ''}`}>
              {isSourceLimited && isTraffic ? '⚠️ 輸出已達上限' : `${data.load.toFixed(0)} QPS`}
            </div>
          )}
        </div>
      </div>
      <Handle
        id="s"
        type="source"
        position={Position.Right}
        isConnectable={!isSourceLimited}
        className={isSourceLimited ? 'handle-limited' : ''}
      />
    </div>
  );
};

const nodeTypes = {
  custom: CustomNode,
};

const edgeTypes = {
  custom: CustomEdge,
};

function App() {
  const [isWasmLoaded, setIsWasmLoaded] = useState(false);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [evaluationResult, setEvaluationResult] = useState(null);
  const [gameTime, setGameTime] = useState(0);
  const [isAutoEvaluating, setIsAutoEvaluating] = useState(false);

  const deleteNode = useCallback((id) => {
    setNodes((nds) => nds.filter((node) => node.id !== id));
    setEdges((eds) => eds.filter((edge) => edge.source !== id && edge.target !== id));
  }, [setNodes, setEdges]);

  const deleteEdge = useCallback((id) => {
    setEdges((eds) => eds.filter((edge) => edge.id !== id));
  }, [setEdges]);

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
        data: { label: '使用者流量', type: 'TRAFFIC_SOURCE', icon: Activity, onDelete: deleteNode },
        deletable: false,
      }
    ];
    setNodes(initialNodes);
  };

  const onConnect = useCallback((params) => {
    const newEdge = {
      ...params,
      type: 'custom',
      data: { onDelete: deleteEdge }
    };
    setEdges((eds) => addEdge(newEdge, eds));
  }, [setEdges, deleteEdge]);

  const addComponent = (type, label, icon, properties = {}) => {
    const id = `${type.toLowerCase()}-${Date.now()}`;
    const newNode = {
      id,
      type: 'custom',
      position: { x: 300, y: 150 },
      data: { label, type, icon, properties, onDelete: deleteNode },
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
      const res = JSON.parse(resultStr);
      setEvaluationResult(res);

      // 動態更新連線動畫：只要系統健康度大於 0 且正在模擬就讓它流動
      const isActive = res.total_score > 0 && isAutoEvaluating;

      setEdges((prevEdges) =>
        prevEdges.map(edge => ({
          ...edge,
          animated: isActive,
          className: isActive ? 'animated' : ''
        }))
      );

      // 同步更新節點狀態（僅針對活躍路徑組件顯示負載）
      setNodes((nds) => nds.map(node => {
        const isActive = res.active_component_ids?.includes(node.id) && isAutoEvaluating;

        if (node.id === 'traffic-1') {
          return { ...node, data: { ...node.data, load: res.total_qps, active: isAutoEvaluating } };
        }

        if (node.data.type === 'WEB_SERVER') {
          // 僅針對活躍的伺服器平攤 QPS
          const activeServersCount = nds.filter(n => n.data.type === 'WEB_SERVER' && res.active_component_ids?.includes(n.id)).length;
          const nodeLoad = isActive ? (res.total_qps / Math.max(1, activeServersCount)) : 0;
          return { ...node, data: { ...node.data, load: nodeLoad, active: isActive } };
        }

        return { ...node, data: { ...node.data, active: isActive, load: isActive ? undefined : 0 } };
      }));
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
            edgeTypes={edgeTypes}
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

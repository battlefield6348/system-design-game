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
  const [isHovered, setIsHovered] = useState(false);
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  return (
    <g
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      className="custom-edge-group"
    >
      {/* 隱形寬路徑：增加滑鼠感應範圍 (25px 寬) */}
      <path
        d={edgePath}
        fill="none"
        stroke="transparent"
        strokeWidth={25}
        className="react-flow__edge-interaction"
        style={{ cursor: 'pointer', pointerEvents: 'all' }}
      />
      {/* 底層實線 */}
      <BaseEdge
        path={edgePath}
        markerEnd={markerEnd}
        style={{ ...style, stroke: isHovered ? '#818cf8' : '#334155', strokeWidth: isHovered ? 4 : 2 }}
      />
      {/* 頂層流動脈衝 */}
      {data?.animated && (
        <BaseEdge
          path={edgePath}
          style={{
            ...style,
            stroke: '#6366f1',
            strokeWidth: 3,
            animationDuration: data.load ? `${Math.max(0.5, 3 - (data.load / 5000))}s` : '1.5s',
            strokeDasharray: data.load ? `5, ${Math.max(20, 100 - (data.load / 100))}` : '10, 90'
          }}
          className="animated"
        />
      )}
      <EdgeLabelRenderer>
        <div
          style={{
            position: 'absolute',
            transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
            pointerEvents: 'all',
            opacity: isHovered ? 1 : 0,
            scale: isHovered ? 1 : 0.5,
            transition: 'all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275)',
            zIndex: 1000,
          }}
          className="nodrag nopan"
        >
          <button className="edge-delete-btn" onClick={() => data.onDelete(id)}>
            <X size={12} strokeWidth={4} />
          </button>
        </div>
      </EdgeLabelRenderer>
    </g>
  );
};

// 自定義節點組件
const CustomNode = ({ data, selected, id }) => {
  const Icon = data.icon || Server;
  const isTraffic = data.type === 'TRAFFIC_SOURCE';
  const isServer = data.type === 'WEB_SERVER';

  // 監控連線數量 (暫時移除以排查白屏問題)
  // const connectionsIn = useHandleConnections({ type: 'target', id: 't' });
  // const connectionsOut = useHandleConnections({ type: 'source', id: 's' });
  const connectionsIn = [];
  const connectionsOut = [];

  const isTargetLimited = isServer && connectionsIn.length >= 1;
  const isSourceLimited = isTraffic && connectionsOut.length >= 1;

  const isOverloaded = data.properties?.max_qps && data.load > data.properties.max_qps;
  const isCrashed = data.crashed;

  return (
    <div className={`custom-node ${data.type.toLowerCase()} ${selected ? 'selected' : ''} ${data.active ? 'active' : ''} ${isOverloaded ? 'overloaded' : ''} ${isCrashed ? 'crashed' : ''}`}>
      {isCrashed && (
        <div className="crashed-overlay">
          <div className="crashed-label">已崩潰</div>
          <button className="restart-btn" onClick={(e) => {
            e.stopPropagation();
            data.onRestart(id);
          }}>
            重啟服務
          </button>
        </div>
      )}
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
        isConnectable={!isTargetLimited && !isCrashed}
        className={isTargetLimited || isCrashed ? 'handle-limited' : ''}
      />
      <div className="node-content">
        <Icon size={20} />
        <div className="node-info">
          <div className="node-name">{data.label}</div>
          <div className="node-type">{data.type}</div>
          {data.load !== undefined && (
            <div className={`node-stats ${isSourceLimited && isTraffic ? 'limited' : ''} ${(data.properties?.max_qps && data.load > data.properties.max_qps) ? 'overloaded' : ''}`}>
              {isCrashed ? '0' : data.load.toFixed(0)}
              {data.properties?.max_qps ? ` / ${data.properties.max_qps}` : ''} QPS
            </div>
          )}
        </div>
      </div>
      <Handle
        id="s"
        type="source"
        position={Position.Right}
        isConnectable={!isSourceLimited && !isCrashed}
        className={isSourceLimited || isCrashed ? 'handle-limited' : ''}
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

  // 支援多選刪除
  const onNodesDelete = useCallback((nodesToDelete) => {
    const nodeIds = new Set(nodesToDelete.map((n) => n.id));
    setNodes((nds) => nds.filter((node) => !nodeIds.has(node.id)));
    setEdges((eds) => eds.filter((edge) => !nodeIds.has(edge.source) && !nodeIds.has(edge.target)));
  }, [setNodes, setEdges]);
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

  // 重啟崩潰節點
  const restartNode = useCallback((id) => {
    setNodes((nds) => nds.map(node => {
      if (node.id === id) {
        return {
          ...node,
          data: {
            ...node.data,
            crashed: false,
            properties: { ...node.data.properties, crashed: false, restartedAt: gameTime }
          }
        };
      }
      return node;
    }));
  }, [setNodes, gameTime]);

  useEffect(() => {
    const go = new window.Go();
    WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject).then((result) => {
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
        data: {
          label: '使用者流量',
          type: 'TRAFFIC_SOURCE',
          icon: Activity,
          onDelete: deleteNode,
          properties: { start_qps: 0 }
        },
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
        prevEdges.map(edge => {
          const targetLoad = res.component_loads?.[edge.target] || 0;
          return {
            ...edge,
            animated: isActive,
            className: isActive ? 'animated' : '',
            data: { ...edge.data, load: targetLoad, onDelete: deleteEdge }
          };
        })
      );

      // 同步更新節點狀態
      setNodes((nds) => nds.map(node => {
        const isPathActive = res.active_component_ids?.includes(node.id);
        const isActiveNode = isPathActive && isAutoEvaluating;
        const isCrashed = res.crashed_component_ids?.includes(node.id);
        const nodeLoad = res.component_loads?.[node.id] || 0;

        return {
          ...node,
          data: {
            ...node.data,
            load: isActiveNode ? nodeLoad : 0,
            active: isActiveNode,
            crashed: isCrashed || node.data.crashed,
            properties: { ...node.data.properties, crashed: isCrashed || node.data.crashed },
            onDelete: deleteNode,
            onRestart: restartNode
          }
        };
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

          <div className="metric-control">
            <span className="metric-label">初始流量:</span>
            <input
              type="number"
              className="metric-input"
              value={nodes.find(n => n.data.type === 'TRAFFIC_SOURCE')?.data.properties?.start_qps || 0}
              onChange={(e) => {
                const val = parseInt(e.target.value) || 0;
                setNodes(nds => nds.map(n => {
                  if (n.data.type === 'TRAFFIC_SOURCE') {
                    return {
                      ...n,
                      data: {
                        ...n.data,
                        properties: { ...n.data.properties, start_qps: val }
                      }
                    };
                  }
                  return n;
                }));
              }}
            />
            <span className="metric-unit">QPS</span>
          </div>

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
            <button onClick={() => addComponent('DATABASE', '資料庫', Database, { max_qps: 500 })}>
              <Plus size={14} /> 資料庫
            </button>
            <button onClick={() => addComponent('CACHE', 'Redis 快取', Activity, { max_qps: 10000 })}>
              <Plus size={14} /> Redis 快取
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
            onNodesDelete={onNodesDelete}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            fitView
            selectionOnDrag={true}
            selectionMode="partial"
          >
            <Background color="#333" gap={20} />
            <Controls />
            <MiniMap
              nodeStrokeWidth={3}
              zoomable
              pannable
              nodeColor="#64748b"
              maskColor="rgba(30, 41, 59, 0.8)"
            />
          </ReactFlow>
        </section>
      </main>
    </div>
  );
}

export default App;

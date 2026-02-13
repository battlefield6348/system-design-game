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
  useReactFlow,
  ReactFlowProvider,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Server, Activity, Database, Share2, Plus, Play, X, List, Globe, Shield, HardDrive, Search, Layout, Copy, RotateCcw, Target, Trophy, ChevronDown, ChevronRight, Users, Zap, ShieldCheck, Waves, Cpu } from 'lucide-react';
import dagre from 'dagre';
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

  // Determine Max QPS to display
  // For DB Cluster, we need to account for multi-node capacity even when simulation is not running
  let baseMaxQPS = data.properties?.max_qps || 0;
  let multiplier = 1;
  if (data.type === 'DATABASE' && data.properties?.replication_mode === 'MASTER_SLAVE') {
    multiplier = 1 + (data.properties.slave_count || 0);
  }

  const displayMaxQPS = (data.active && data.effectiveMaxQPS)
    ? data.effectiveMaxQPS
    : (baseMaxQPS * multiplier);

  const isOverloaded = displayMaxQPS > 0 && data.load > displayMaxQPS;
  const isCrashed = data.crashed;
  const isBursting = data.isBurstActive && isTraffic;
  const isASG = data.type === 'AUTO_SCALING_GROUP';

  // For ASG, we render multiple server instances
  const replicas = data.replicas || 1;
  const instances = Array.from({ length: replicas });

  return (
    <div className={`custom-node ${data.type.toLowerCase()} ${selected ? 'selected' : ''} ${data.active ? 'active' : ''} ${isOverloaded ? 'overloaded' : ''} ${isCrashed ? 'crashed' : ''} ${isBursting ? 'bursting' : ''} ${isASG ? 'asg-container' : ''}`}>
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
        {isASG ? (
          <div className="asg-container-inner">
            <div className="asg-header">
              <div className="asg-title">{data.label}</div>
              <div className="asg-badge">AUTO SCALING GROUP</div>
            </div>

            <div className="asg-body">
              {/* 繪製內部自動佈線 */}
              <svg className="asg-wiring" viewBox="0 0 310 100" preserveAspectRatio="none">
                {instances.map((_, i) => {
                  const isFirst = i === 0;
                  const isProvisioning = !isFirst && data.properties?.replica_start_times &&
                    (data.active_time - data.properties.replica_start_times[i - 1] < (data.properties.warmup_seconds || 10));
                  const yPos = (100 / (replicas + 1)) * (i + 1);
                  return (
                    <g key={`wire-${i}`}>
                      <path
                        d={`M 0,50 C 30,50 40,${yPos} 60,${yPos}`}
                        className={`wire-path ${data.active && !isProvisioning ? 'active' : ''} ${isProvisioning ? 'provisioning' : ''}`}
                      />
                      <path
                        d={`M 250,${yPos} C 270,${yPos} 280,50 310,50`}
                        className={`wire-path ${data.active && !isProvisioning ? 'active' : ''}`}
                      />
                    </g>
                  );
                })}
              </svg>

              <div className="asg-instance-grid">
                {instances.map((_, i) => {
                  const isFirst = i === 0;
                  const isProvisioning = !isFirst && data.properties?.replica_start_times &&
                    (data.active_time - data.properties.replica_start_times[i - 1] < (data.properties.warmup_seconds || 10));

                  // 計算單台分流負載 (只分給已啟動的機器)
                  const workingReplicas = replicas - (data.properties?.replica_start_times?.filter((startTime) => {
                    return (data.active_time - startTime < (data.properties.warmup_seconds || 10));
                  }).length || 0);

                  const individualLoad = data.load / Math.max(1, workingReplicas);
                  const currentFullfilledLoad = data.active && !isProvisioning ? Math.max(0, individualLoad) : 0;

                  return (
                    <div key={i} className={`server-card ${data.active && !isProvisioning ? 'active' : ''} ${isProvisioning ? 'provisioning' : ''}`}>
                      <Server size={18} />
                      <div className="server-info">
                        <span className="server-id">Node {i + 1}</span>
                        <div className="server-qps-pill">{(currentFullfilledLoad || 0).toFixed(0)} QPS</div>
                      </div>
                      {isProvisioning && <div className="booting-spinner"></div>}
                    </div>
                  );
                })}
              </div>
            </div>

            <div className="asg-footer">
              <div className={`node-stats ${isOverloaded ? 'overloaded' : ''}`}>
                總容量: {displayMaxQPS} QPS ({(data.properties?.max_qps || 1000)} x {data.replicas || 1})
              </div>
              <div className={`node-stats ${isOverloaded ? 'overloaded' : ''}`} style={{ borderTop: 'none', paddingTop: 0, marginTop: 2 }}>
                總負載: {(data.load || 0).toFixed(0)} QPS
              </div>
            </div>
          </div>
        ) : (
          <>
            {data.type === 'DATABASE' && data.properties?.replication_mode === 'MASTER_SLAVE' ? (
              <div className="db-ms-cluster">
                <div className="db-master">
                  <Database size={32} />
                  <span className="db-label">Master</span>
                </div>
                <div className="db-slaves">
                  {[...Array(data.properties.slave_count || 1)].map((_, i) => (
                    <div key={i} className="db-slave">
                      <Database size={16} />
                      <span className="db-label">Slave</span>
                    </div>
                  ))}
                </div>
                <div className="db-cluster-stats">
                  容量: {displayMaxQPS} QPS ({baseMaxQPS} x {multiplier})
                </div>
              </div>
            ) : (
              <div className="node-icon-wrapper">
                <Icon size={40} />
              </div>
            )}
            <div className="node-info">
              <div className="node-name">{data.label}</div>
              <div className="node-type">{data.type}</div>
              {data.load !== undefined && (
                <div className={`node-stats ${isSourceLimited && isTraffic ? 'limited' : ''} ${isOverloaded ? 'overloaded' : ''}`}>
                  {isCrashed ? '0' : (data.load || 0).toFixed(0)}
                  {displayMaxQPS ? ` / ${displayMaxQPS}` : ''} QPS
                </div>
              )}
              {data.malicious_load > 0 && (
                <div className="node-stats" style={{ borderTop: 'none', paddingTop: 0, color: '#ef4444', fontWeight: 'bold' }}>
                  ☢ 惡意: {data.malicious_load.toFixed(0)} QPS
                </div>
              )}
              {data.type === 'MESSAGE_QUEUE' && (
                <div className={`node-stats ${data.properties?.backlog > 0 ? 'limited' : ''}`} style={{ borderTop: 'none', paddingTop: 0 }}>
                  積壓: {Math.max(0, data.properties?.backlog || 0).toFixed(0)} Msg
                </div>
              )}
            </div>
          </>
        )}
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

const getLayoutedElements = (nodes, edges, direction = 'LR') => {
  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));

  const isHorizontal = direction === 'LR';
  dagreGraph.setGraph({ rankdir: direction });

  nodes.forEach((node) => {
    const isASG = node.data.type === 'AUTO_SCALING_GROUP';
    const replicas = node.data.replicas || 1;
    // 動態計算高度：Header(40) + Footer(60) + (每台機器高度+Gap 約 65)
    const asgHeight = isASG ? Math.max(180, 100 + replicas * 65) : 120;
    dagreGraph.setNode(node.id, { width: isASG ? 310 : 260, height: asgHeight });
  });

  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  dagre.layout(dagreGraph);

  const newNodes = nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    const newNode = {
      ...node,
      targetPosition: isHorizontal ? 'left' : 'top',
      sourcePosition: isHorizontal ? 'right' : 'bottom',
      // We are shifting the dagre node position (anchor=center center) to the top left
      // so it matches the React Flow node anchor point (top left).
      position: {
        x: nodeWithPosition.x - (node.data.type === 'AUTO_SCALING_GROUP' ? 310 : 260) / 2,
        y: nodeWithPosition.y - (node.data.type === 'AUTO_SCALING_GROUP' ? Math.max(180, 100 + (node.data.replicas || 1) * 65) : 120) / 2,
      },
    };

    return newNode;
  });

  return { nodes: newNodes, edges };
};

function Game() {
  const [isWasmLoaded, setIsWasmLoaded] = useState(false);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const { fitView } = useReactFlow();

  // 支援多選刪除
  const onNodesDelete = useCallback((nodesToDelete) => {
    const nodeIds = new Set(nodesToDelete.map((n) => n.id));
    setNodes((nds) => nds.filter((node) => !nodeIds.has(node.id)));
    setEdges((eds) => eds.filter((edge) => !nodeIds.has(edge.source) && !nodeIds.has(edge.target)));
  }, [setNodes, setEdges]);
  const [evaluationResult, setEvaluationResult] = useState(null);
  const [gameTime, setGameTime] = useState(0);
  const [isAutoEvaluating, setIsAutoEvaluating] = useState(false);
  const [retentionRate, setRetentionRate] = useState(1.0);
  const [scenarios, setScenarios] = useState([]);
  const [selectedScenario, setSelectedScenario] = useState(null);
  const [showScenarioModal, setShowScenarioModal] = useState(false);

  const [expandedCategories, setExpandedCategories] = useState({
    compute: true,
    networking: true,
    storage: false,
    middleware: false
  });

  const [hoveredTool, setHoveredTool] = useState(null);
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });

  const toggleCategory = (cat) => {
    setExpandedCategories(prev => ({ ...prev, [cat]: !prev[cat] }));
  };

  const toolDescriptions = {
    'NANO_SERVER': '入門級節點 (200 QPS)。適合初期流量或輕量服務，部署成本極低。',
    'STANDARD_SERVER': '標準運算節點 (1k QPS)。效能平衡點，適合大多數業務邏輯處理。',
    'HIGH_PERF_SERVER': '最強效能節點 (5k QPS)。雖然昂貴，但在處理複雜計算與高併發時最為穩定。',
    'AUTO_SCALING_GROUP': '自動擴縮容叢集。能根據 CPU 或負載自動增減機器數量，應對突發流量的首選。',
    'LOAD_BALANCER': '流量分發器。確保後端伺服器負載均衡，避免單點故障。',
    'CDN': '全球邊緣快取。緩存靜態資源與圖片，能擋掉 80% 以上的回源請求。',
    'WAF': '網路防火牆。能識別並攔截惡意攻擊，提升系統安全性評分。',
    'DATABASE': '關聯式資料庫 (PostgreSQL)。儲存結構化數據，高負載下需要考慮 Replication。',
    'OBJECT_STORAGE': '雲端儲存 (S3)。專門存放影音、Log 等大型文件，具備極高的可用性。',
    'SEARCH_ENGINE': '搜尋引擎 (ES)。解決資料庫在全文檢索下的效能瓶頸，熱搜必備。',
    'CACHE': '極速快取 (Redis)。將熱點數據放進內容中，讓 API 延遲縮短至 1ms 以內。',
    'MESSAGE_QUEUE': '異步訊息隊列 (Kafka)。讓系統組件解耦，具備削峰填谷能力。'
  };

  // 初始化取得關卡列表
  useEffect(() => {
    if (isWasmLoaded && window.goListScenarios) {
      try {
        const res = JSON.parse(window.goListScenarios());
        setScenarios(res);
        if (res.length > 0 && !selectedScenario) {
          setSelectedScenario(res[0]);
        }
      } catch (e) {
        console.error("Failed to fetch scenarios", e);
      }
    }
  }, [isWasmLoaded, selectedScenario]);

  const resetSimulation = () => {
    setGameTime(0);
    setIsAutoEvaluating(false);
    setEvaluationResult(null);
    setNodes((nds) => nds.map(node => ({
      ...node,
      data: {
        ...node.data,
        load: 0,
        malicious_load: 0,
        active: false,
        crashed: false,
        properties: {
          ...node.data.properties,
          backlog: 0,
          crashed: false,
          restartedAt: undefined,
          replica_start_times: []
        }
      }
    })));
  };

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

  const [clipboardNode, setClipboardNode] = useState(null);

  const onCopy = useCallback(() => {
    const selectedNode = nodes.find((node) => node.selected);
    if (selectedNode) {
      setClipboardNode(selectedNode);
    }
  }, [nodes]);

  const onPaste = useCallback(() => {
    if (!clipboardNode) return;

    const id = `${clipboardNode.data.type?.toLowerCase()}-${Date.now()}`;
    const newNode = {
      ...clipboardNode,
      id,
      selected: true,
      position: {
        x: clipboardNode.position.x + 40,
        y: clipboardNode.position.y + 40,
      },
      data: {
        ...clipboardNode.data,
        onDelete: deleteNode,
        onRestart: restartNode,
      },
    };

    // 取消其他節點的選取，並加入新節點
    setNodes((nds) => nds.map((node) => ({ ...node, selected: false })).concat(newNode));
  }, [clipboardNode, deleteNode, restartNode, setNodes]);

  // 全域快捷鍵監聽
  useEffect(() => {
    const handleKeyDown = (e) => {
      const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
      const modifier = isMac ? e.metaKey : e.ctrlKey;

      // 只有在非輸入框狀態下才觸發
      if (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA' || document.activeElement.tagName === 'SELECT') {
        return;
      }

      if (modifier && e.key === 'c') {
        e.preventDefault();
        onCopy();
      }
      if (modifier && e.key === 'v') {
        e.preventDefault();
        onPaste();
      }
      if (e.key === 'Delete' || e.key === 'Backspace') {
        const selectedNodes = nodes.filter(n => n.selected);
        if (selectedNodes.length > 0) {
          onNodesDelete(selectedNodes);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onCopy, onPaste, nodes, onNodesDelete]);


  const onLayout = useCallback((direction) => {
    const { nodes: layoutedNodes, edges: layoutedEdges } = getLayoutedElements(
      nodes,
      edges,
      direction,
    );

    setNodes([...layoutedNodes]);
    setEdges([...layoutedEdges]);

    window.requestAnimationFrame(() => {
      fitView();
    });
  }, [nodes, edges, setNodes, setEdges, fitView]);

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
  }, [isWasmLoaded, isAutoEvaluating, nodes, edges, retentionRate]); // Added retentionRate to deps

  const initDefaultDesign = () => {
    const initialNodes = [
      {
        id: 'traffic-1',
        type: 'custom',
        position: { x: 50, y: 150 },
        data: {
          label: '使用者流量',
          type: 'TRAFFIC_SOURCE',
          icon: Users,
          onDelete: deleteNode,
          properties: { start_qps: 0 }
        },
        deletable: false,
      }
    ];
    setNodes(initialNodes);
  };

  const onConnect = useCallback((params) => {
    // 檢查連線規則：除了 LB，禁止其他組件連向 ASG
    const sourceNode = nodes.find(n => n.id === params.source);
    const targetNode = nodes.find(n => n.id === params.target);

    if (targetNode?.data.type === 'AUTO_SCALING_GROUP' && sourceNode?.data.type !== 'LOAD_BALANCER') {
      alert('架構規範：彈性伸縮組 (ASG) 前端必須接負載平衡器 (LB)！');
      return;
    }

    // 限制流量來源只能有一條連出線路
    if (sourceNode?.data.type === 'TRAFFIC_SOURCE') {
      const existingEdges = edges.filter(e => e.source === params.source);
      if (existingEdges.length >= 1) {
        alert('架構規範：流量來源 (Traffic Source) 只能連向一個進入點。如需分流，請接上 Load Balancer！');
        return;
      }
    }

    const newEdge = {
      ...params,
      type: 'custom',
      data: { onDelete: deleteEdge }
    };
    setEdges((eds) => addEdge(newEdge, eds));
  }, [setEdges, deleteEdge, nodes, edges]);

  const addComponent = (type, label, icon, properties = {}) => {
    const id = `${type.toLowerCase()}-${Date.now()}`;
    const newNode = {
      id,
      type: 'custom',
      position: { x: 300, y: 150 },
      data: {
        label, type, icon, properties,
        setup_cost: properties.setup_cost || 0,
        operational_cost: properties.operational_cost || 0,
        onDelete: deleteNode
      },
    };
    setNodes((nds) => nds.concat(newNode));
  };

  const handleEvaluate = (currentTime = gameTime) => {
    if (!window.goSaveDesign || !window.goEvaluate) return;

    // 將 React Flow 狀態轉換為 Go 領域模型
    const design = {
      id: "live-design",
      scenario_id: selectedScenario?.id || "tinyurl",
      components: nodes.map(n => ({
        id: n.id,
        name: n.data.label,
        type: n.data.type,
        setup_cost: n.data.setup_cost || 0,
        operational_cost: n.data.operational_cost || 0,
        properties: n.data.properties || { max_qps: 1000 }
      })),
      connections: edges.map(e => ({
        from_id: e.source,
        to_id: e.target,
        protocol: "HTTP"
      })),
      properties: { retention_rate: retentionRate }
    };

    // 同步到 Wasm
    window.goSaveDesign(JSON.stringify(design));

    // 執行評估 (傳入當前遊戲秒數)
    const resultStr = window.goEvaluate("live-design", currentTime);
    try {
      const res = JSON.parse(resultStr);
      setEvaluationResult(res);

      // 使用者留存率衰減與恢復邏輯
      setRetentionRate(prev => {
        let next = prev;
        if (res.total_score < 95) {
          // 系統健康度低於 95%，使用者開始流失 (-0.5% / sec)
          next -= 0.005;
        } else {
          // 系統健康度恢復，使用者慢慢回歸 (+0.2% / sec)
          next += 0.002;
        }
        return Math.min(1.0, Math.max(0.1, next));
      });

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

      // 同步更新節點狀態並管理 ASG 擴展
      setNodes((nds) => nds.map(node => {
        const isPathActive = res.active_component_ids?.includes(node.id);
        const isActiveNode = isPathActive && isAutoEvaluating;
        const isCrashed = res.crashed_component_ids?.includes(node.id);
        const nodeLoad = res.component_loads?.[node.id] || 0;
        const effectiveMaxQPS = res.component_effective_max_qps?.[node.id] || 0;
        const nodeReplicas = res.component_replicas?.[node.id] || 1;

        let updatedProperties = { ...node.data.properties, crashed: isCrashed || node.data.crashed };

        // ASG 擴展邏輯：記錄新副本的啟動時間
        if (node.data.type === 'AUTO_SCALING_GROUP' && node.data.properties?.auto_scaling) {
          const threshold = (node.data.properties.scale_up_threshold || 70) / 100.0;
          const baseCap = node.data.properties.max_qps || 1000;

          // 計算預期副本數
          const needed = Math.ceil(nodeLoad / (baseCap * threshold)) || 1;
          const max = node.data.properties.max_replicas || 5;
          const target = Math.min(needed, max);

          let startTimes = node.data.properties.replica_start_times || [];
          // 如果目標增加，則新增啟動時間 (扣除第1台基礎機器)
          if (target > 1 && startTimes.length < target - 1) {
            startTimes = [...startTimes, res.created_at];
          }
          // 如果負載下降，縮減副本 (Scale In)
          else if (target < 1 + startTimes.length) {
            startTimes = startTimes.slice(0, target - 1);
          }
          updatedProperties.replica_start_times = startTimes;
        }

        // MQ 積壓同步
        if (node.data.type === 'MESSAGE_QUEUE') {
          updatedProperties.backlog = res.component_backlogs?.[node.id] || 0;
        }

        return {
          ...node,
          data: {
            ...node.data,
            load: isActiveNode ? nodeLoad : 0,
            malicious_load: res.component_malicious_loads?.[node.id] || 0,
            active: isActiveNode,
            active_time: res.created_at, // 用於判斷暖機進度
            isBurstActive: res.is_burst_active,
            crashed: isCrashed || node.data.crashed,
            effectiveMaxQPS: effectiveMaxQPS,
            replicas: nodeReplicas,
            properties: updatedProperties,
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
              <span className="metric" title="成功獲取資料的請求比例">成功率: {(evaluationResult.total_score || 0).toFixed(1)}%</span>
              <span className="metric">取得資料: {evaluationResult.fulfilled_qps} / {evaluationResult.total_qps} QPS</span>
            </div>
          )}

          {evaluationResult?.is_burst_active && (
            <div className="burst-badge">BURSTING!</div>
          )}

          {evaluationResult?.is_random_drop && (
            <div className="drop-badge">UNSTABLE!</div>
          )}

          <button
            className="btn-primary"
            onClick={() => setShowScenarioModal(true)}
            title="選擇挑戰情境"
          >
            <Target size={16} /> {selectedScenario?.title || '選擇場景'}
          </button>

          <div className="btn-group">
            <button
              className="btn-primary"
              onClick={() => onLayout('LR')}
              title="自動排版 (Auto Layout)"
            >
              <Layout size={16} /> 排版
            </button>

            <button
              className={`btn-primary ${isAutoEvaluating ? 'active' : 'warning'}`}
              onClick={() => setIsAutoEvaluating(!isAutoEvaluating)}
              disabled={!isWasmLoaded}
            >
              <Play size={16} /> {isAutoEvaluating ? '暫停' : '開始'}
            </button>

            <button
              className="btn-primary danger"
              onClick={resetSimulation}
              disabled={!isWasmLoaded}
              title="重置所有流量與狀態"
            >
              <RotateCcw size={16} /> 重置
            </button>
          </div>
        </div>
      </header>

      <main className="game-main">
        <aside className="tool-panel">
          {nodes.find(n => n.selected) ? (
            <div className="property-editor">
              <div className="property-header">
                <h3>組件設定</h3>
                <button className="btn-icon-sm" onClick={onCopy} title="複製組件 (Cmd+C)">
                  <Copy size={16} />
                </button>
              </div>
              {(() => {
                const selectedNode = nodes.find(n => n.selected);
                const isASG = selectedNode?.data.type === 'AUTO_SCALING_GROUP';
                return (
                  <div className="props-form">
                    <div className="prop-group">
                      <label>名稱</label>
                      <input
                        type="text"
                        value={selectedNode.data.label}
                        onChange={(e) => {
                          setNodes(nds => nds.map(n => {
                            if (n.id === selectedNode.id) {
                              return { ...n, data: { ...n.data, label: e.target.value } };
                            }
                            return n;
                          }));
                        }}
                      />
                    </div>
                    {selectedNode.data.properties?.max_qps !== undefined && (
                      <div className="prop-group">
                        <label>
                          {isASG || selectedNode.data.type === 'WEB_SERVER' ? '單機處理能力 (Max QPS per Node)' : '處理能力 (Max QPS)'}
                        </label>
                        <input
                          type="number"
                          value={selectedNode.data.properties.max_qps}
                          onChange={(e) => {
                            const val = parseInt(e.target.value) || 0;
                            setNodes(nds => nds.map(n => {
                              if (n.id === selectedNode.id) {
                                return {
                                  ...n,
                                  data: {
                                    ...n.data,
                                    properties: { ...n.data.properties, max_qps: val }
                                  }
                                };
                              }
                              return n;
                            }));
                          }}
                        />
                      </div>
                    )}

                    {/* Database Replication Settings */}
                    {selectedNode.data.type === 'DATABASE' && (
                      <>
                        <div className="prop-group">
                          <label>部署模式 (Deployment Mode)</label>
                          <select
                            className="metric-input"
                            style={{ width: '100%', background: 'rgba(255,255,255,0.05)', padding: '8px' }}
                            value={selectedNode.data.properties.replication_mode || 'SINGLE'}
                            onChange={(e) => {
                              const val = e.target.value;
                              setNodes(nds => nds.map(n => {
                                if (n.id === selectedNode.id) {
                                  return {
                                    ...n,
                                    data: {
                                      ...n.data,
                                      properties: {
                                        ...n.data.properties,
                                        replication_mode: val,
                                        slave_count: val === 'MASTER_SLAVE' ? 1 : 0
                                      }
                                    }
                                  };
                                }
                                return n;
                              }));
                            }}
                          >
                            <option value="SINGLE">單機 (Single)</option>
                            <option value="MASTER_SLAVE">主從架構 (Master-Slave)</option>
                          </select>
                        </div>
                        {selectedNode.data.properties.replication_mode === 'MASTER_SLAVE' && (
                          <div className="prop-group">
                            <label>從庫數量 (Slave Count: {selectedNode.data.properties.slave_count || 1})</label>
                            <input
                              type="range" min="1" max="3"
                              value={selectedNode.data.properties.slave_count || 1}
                              onChange={(e) => {
                                const val = parseInt(e.target.value);
                                setNodes(nds => nds.map(n => {
                                  if (n.id === selectedNode.id) {
                                    return {
                                      ...n,
                                      data: {
                                        ...n.data,
                                        properties: { ...n.data.properties, slave_count: val }
                                      }
                                    };
                                  }
                                  return n;
                                }));
                              }}
                            />
                          </div>
                        )}
                      </>
                    )}

                    {/* Auto Scaling Logic ONLY for ASG */}
                    {selectedNode.data.type === 'AUTO_SCALING_GROUP' && (
                      <>
                        <div className="prop-group checkbox">
                          <label>
                            <input
                              type="checkbox"
                              checked={selectedNode.data.properties.auto_scaling || false}
                              onChange={(e) => {
                                setNodes(nds => nds.map(n => {
                                  if (n.id === selectedNode.id) {
                                    return {
                                      ...n,
                                      data: {
                                        ...n.data,
                                        properties: { ...n.data.properties, auto_scaling: e.target.checked }
                                      }
                                    };
                                  }
                                  return n;
                                }));
                              }}
                            />
                            啟用 Auto Scaling
                          </label>
                        </div>
                        {selectedNode.data.properties.auto_scaling && (
                          <>
                            <div className="prop-group">
                              <label>最大副本數 (Max Replicas)</label>
                              <input
                                type="number"
                                value={selectedNode.data.properties.max_replicas || 5}
                                onChange={(e) => {
                                  const val = parseInt(e.target.value) || 1;
                                  setNodes(nds => nds.map(n => {
                                    if (n.id === selectedNode.id) {
                                      return {
                                        ...n,
                                        data: {
                                          ...n.data,
                                          properties: { ...n.data.properties, max_replicas: val }
                                        }
                                      };
                                    }
                                    return n;
                                  }));
                                }}
                              />
                            </div>
                            <div className="prop-group">
                              <label>擴展門檻 (Scaling Threshold: {selectedNode.data.properties.scale_up_threshold || 70}%)</label>
                              <input
                                type="range" min="10" max="95"
                                value={selectedNode.data.properties.scale_up_threshold || 70}
                                onChange={(e) => {
                                  const val = parseInt(e.target.value);
                                  setNodes(nds => nds.map(n => {
                                    if (n.id === selectedNode.id) {
                                      return {
                                        ...n,
                                        data: {
                                          ...n.data,
                                          properties: { ...n.data.properties, scale_up_threshold: val }
                                        }
                                      };
                                    }
                                    return n;
                                  }));
                                }}
                              />
                            </div>
                            <div className="prop-group">
                              <label>暖機時間 (Warm-up: {selectedNode.data.properties.warmup_seconds || 10}s)</label>
                              <input
                                type="number"
                                value={selectedNode.data.properties.warmup_seconds || 10}
                                onChange={(e) => {
                                  const val = parseInt(e.target.value) || 1;
                                  setNodes(nds => nds.map(n => {
                                    if (n.id === selectedNode.id) {
                                      return {
                                        ...n,
                                        data: {
                                          ...n.data,
                                          properties: { ...n.data.properties, warmup_seconds: val }
                                        }
                                      };
                                    }
                                    return n;
                                  }));
                                }}
                              />
                            </div>
                          </>
                        )}
                      </>
                    )}

                    {/* Burst Traffic Logic for Traffic Source */}
                    {selectedNode.data.type === 'TRAFFIC_SOURCE' && (
                      <div className="props-form">
                        <div className="prop-group">
                          <label>初始 QPS (Start QPS)</label>
                          <input
                            type="number"
                            value={selectedNode.data.properties.start_qps || 0}
                            onChange={(e) => {
                              const val = parseInt(e.target.value) || 0;
                              setNodes(nds => nds.map(n => {
                                if (n.id === selectedNode.id) {
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
                        </div>
                        <div className="prop-group checkbox">
                          <label>
                            <input
                              type="checkbox"
                              checked={selectedNode.data.properties.burst_traffic || false}
                              onChange={(e) => {
                                setNodes(nds => nds.map(n => {
                                  if (n.id === selectedNode.id) {
                                    return {
                                      ...n,
                                      data: {
                                        ...n.data,
                                        properties: { ...n.data.properties, burst_traffic: e.target.checked }
                                      }
                                    };
                                  }
                                  return n;
                                }));
                              }}
                            />
                            啟用突發流量 (Simulate Spikes)
                          </label>
                        </div>
                      </div>
                    )}

                    {/* MQ Specific Settings */}
                    {selectedNode.data.type === 'MESSAGE_QUEUE' && (
                      <div className="prop-group">
                        <label>傳輸模式 (Delivery Mode)</label>
                        <select
                          className="metric-input"
                          style={{ width: '100%', padding: '0.5rem', marginBottom: '0.5rem' }}
                          value={selectedNode.data.properties.delivery_mode || 'PUSH'}
                          onChange={(e) => {
                            setNodes(nds => nds.map(n => {
                              if (n.id === selectedNode.id) {
                                return {
                                  ...n,
                                  data: {
                                    ...n.data,
                                    properties: { ...n.data.properties, delivery_mode: e.target.value }
                                  }
                                };
                              }
                              return n;
                            }));
                          }}
                        >
                          <option value="PUSH">Push (主動推送)</option>
                          <option value="PULL">Pull (被動拉取)</option>
                        </select>
                      </div>
                    )}

                    <button className="btn-secondary" onClick={() => setNodes(nds => nds.map(n => ({ ...n, selected: false })))}>
                      關閉設定
                    </button>
                    <div className="help-text">
                      {selectedNode.data.type === 'MESSAGE_QUEUE' && "提示：調整 QPS 來控制給下游的流量速度 (削峰填谷)。"}
                      {selectedNode.data.type === 'WEB_SERVER' && "提示：單機 QPS 上限，超過會導致崩潰或延遲。"}
                      {selectedNode.data.type === 'CDN' && "提示：CDN 可快取靜態資源，大幅降低 Origin 負載 (約 80%)。"}
                      {selectedNode.data.type === 'WAF' && "提示：WAF 用於過濾惡意流量，保護後端安全。"}
                      {selectedNode.data.type === 'OBJECT_STORAGE' && "提示：高持久性的物件儲存服務 (如 S3)，幾乎不會崩潰。"}
                      {selectedNode.data.type === 'SEARCH_ENGINE' && "提示：專門處理全文搜索請求，比資料庫更適合大量讀取。"}
                    </div>
                  </div>
                );
              })()}
            </div>
          ) : (
            <div className="tool-accordion">
              <div className={`tool-category ${expandedCategories.compute ? 'expanded' : ''}`}>
                <div className="category-header" onClick={() => toggleCategory('compute')}>
                  <h3>Compute 運算節點</h3>
                  {expandedCategories.compute ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </div>
                {expandedCategories.compute && (
                  <div className="tool-list drawer-content">
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'Nano Server', type: 'NANO_SERVER' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('WEB_SERVER', 'Nano Server', Cpu, { max_qps: 200, base_latency: 100, setup_cost: 50, operational_cost: 0.05 })}
                    >
                      <Plus size={14} /> Nano Server
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: '標準伺服器', type: 'STANDARD_SERVER' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('WEB_SERVER', '標準伺服器', Server, { max_qps: 1000, base_latency: 50, setup_cost: 200, operational_cost: 0.2 })}
                    >
                      <Plus size={14} /> 標準伺服器
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: '高效能伺服器', type: 'HIGH_PERF_SERVER' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('WEB_SERVER', '高效能伺服器', Activity, { max_qps: 5000, base_latency: 20, setup_cost: 800, operational_cost: 0.7 })}
                    >
                      <Plus size={14} /> 高效能伺服器
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'ASG 叢集', type: 'AUTO_SCALING_GROUP' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('AUTO_SCALING_GROUP', '彈性伸縮組 (ASG)', Layout, { max_qps: 1000, auto_scaling: true, max_replicas: 5, scale_up_threshold: 70, warmup_seconds: 10, operational_cost: 0.3 })}
                    >
                      <Plus size={14} /> ASG 叢集
                    </button>
                  </div>
                )}
              </div>

              <div className={`tool-category ${expandedCategories.networking ? 'expanded' : ''}`}>
                <div className="category-header" onClick={() => toggleCategory('networking')}>
                  <h3>Networking 網路設施</h3>
                  {expandedCategories.networking ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </div>
                {expandedCategories.networking && (
                  <div className="tool-list drawer-content">
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: '負載平衡器', type: 'LOAD_BALANCER' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('LOAD_BALANCER', '負載平衡器', Zap, { max_qps: 20000, base_latency: 5, operational_cost: 0.1 })}
                    >
                      <Plus size={14} /> 負載平衡器
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'CDN 傳遞', type: 'CDN' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('CDN', 'CDN (全球快取)', Globe, { max_qps: 50000 })}
                    >
                      <Plus size={14} /> CDN 傳遞
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'WAF 防火牆', type: 'WAF' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('WAF', 'WAF (防火牆)', ShieldCheck, { max_qps: 20000 })}
                    >
                      <Plus size={14} /> WAF 防火牆
                    </button>
                  </div>
                )}
              </div>

              <div className={`tool-category ${expandedCategories.storage ? 'expanded' : ''}`}>
                <div className="category-header" onClick={() => toggleCategory('storage')}>
                  <h3>Storage 資料儲存</h3>
                  {expandedCategories.storage ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </div>
                {expandedCategories.storage && (
                  <div className="tool-list drawer-content">
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'SQL 資料庫', type: 'DATABASE' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('DATABASE', '資料庫 (RDB)', Database, { max_qps: 500, replication_mode: 'SINGLE', slave_count: 0, base_latency: 50, operational_cost: 0.5 })}
                    >
                      <Plus size={14} /> SQL 資料庫
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'S3 儲存', type: 'OBJECT_STORAGE' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('OBJECT_STORAGE', '物件儲存 (S3)', HardDrive, { max_qps: 100000 })}
                    >
                      <Plus size={14} /> S3 儲存
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'ElasticSearch', type: 'SEARCH_ENGINE' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('SEARCH_ENGINE', '搜尋引擎 (ES)', Search, { max_qps: 2000 })}
                    >
                      <Plus size={14} /> ElasticSearch
                    </button>
                  </div>
                )}
              </div>

              <div className={`tool-category ${expandedCategories.middleware ? 'expanded' : ''}`}>
                <div className="category-header" onClick={() => toggleCategory('middleware')}>
                  <h3>Middleware 中介軟體</h3>
                  {expandedCategories.middleware ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </div>
                {expandedCategories.middleware && (
                  <div className="tool-list drawer-content">
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'Redis 快取', type: 'CACHE' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('CACHE', 'Redis 快取', Activity, { max_qps: 20000, base_latency: 1, operational_cost: 0.3 })}
                    >
                      <Plus size={14} /> Redis 快取
                    </button>
                    <button
                      onMouseEnter={(e) => {
                        setHoveredTool({ name: 'Kafka 隊列', type: 'MESSAGE_QUEUE' });
                        setMousePos({ x: e.clientX, y: e.clientY });
                      }}
                      onMouseMove={(e) => setMousePos({ x: e.clientX, y: e.clientY })}
                      onMouseLeave={() => setHoveredTool(null)}
                      onClick={() => addComponent('MESSAGE_QUEUE', '訊息佇列 (Kafka)', Waves, { max_qps: 10000, base_latency: 200, operational_cost: 0.4 })}
                    >
                      <Plus size={14} /> Kafka 隊列
                    </button>
                  </div>
                )}
              </div>

            </div>
          )}

          {evaluationResult && (
            <div className="eval-details">
              <h4>系統診斷報告</h4>
              <div className="global-stats">
                <div className="stat-row">
                  <span className="dim">使用者滿意度</span>
                  <span className={`val ${(retentionRate * 100) < 80 ? 'warning' : ''}`}>
                    {((retentionRate || 0) * 100).toFixed(1)}%
                  </span>
                </div>
                {(retentionRate * 100) < 95 && (retentionRate * 100) >= 10 && (
                  <p className="churn-hint">提示：系統不穩定導致使用者流失中...</p>
                )}
              </div>
              {evaluationResult.scores.map((s, i) => (
                <div key={i} className="score-item">
                  <span className="dim">{s.dimension}</span>
                  <span className="val">{(s.value || 0).toFixed(0)}</span>
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
            selectionOnDrag={true} // 允許左鍵直接框選
            selectionKeyCode={null} // 不需要按住任何鍵即可框選
            panOnDrag={[1, 2]} // 改為中鍵或右鍵平移畫布，左鍵留給框選
            panOnScroll={true} // 允許滾輪平移
            selectionMode="partial"
          >
            <Background color="#334155" variant="dots" />
            <Controls />
            <MiniMap
              style={{ height: 120, width: 150 }}
              nodeColor={(n) => {
                if (n.data.type === 'TRAFFIC_SOURCE') return '#f43f5e';
                if (n.data.type === 'DATABASE') return '#10b981';
                return '#6366f1';
              }}
              maskColor="rgba(0, 0, 0, 0.3)"
              zoomable
              pannable
            />
            {/* 右下角重置漂浮按鈕 */}
            <div className="fab-container">
              <button
                className="fab-reset"
                onClick={resetSimulation}
                title="重置模擬狀態"
              >
                <RotateCcw size={20} />
                <span>重置流量</span>
              </button>
            </div>
          </ReactFlow>
        </section>
      </main>

      {/* Scenario Selection Modal */}
      {showScenarioModal && (
        <div className="modal-overlay">
          <div className="scenario-modal">
            <div className="modal-header">
              <h2>選擇系統設計挑戰</h2>
              <button onClick={() => setShowScenarioModal(false)}><X /></button>
            </div>
            <div className="scenario-list">
              {scenarios.map(s => (
                <div
                  key={s.id}
                  className={`scenario-card ${selectedScenario?.id === s.id ? 'active' : ''}`}
                  onClick={() => {
                    setSelectedScenario(s);
                    setShowScenarioModal(false);
                    resetSimulation();
                  }}
                >
                  <div className="card-header">
                    <h4>{s.title}</h4>
                    {selectedScenario?.id === s.id && <Trophy size={16} color="var(--warning)" />}
                  </div>
                  <p>{s.description}</p>
                  <div className="card-goals">
                    <span>目標 QPS: {(s.goal.min_qps / 1000).toFixed(0)}k</span>
                    <span>允許可忍受延遲: {s.goal.max_latency_ms}ms</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Goal Overlay */}
      {selectedScenario && (
        <div className="goal-overlay">
          <div className="goal-title">當前挑戰: {selectedScenario.title}</div>
          <div className="goal-progress">
            目標 QPS: {(selectedScenario.goal.min_qps / 1000).toFixed(0)}k |
            最大延遲: {selectedScenario.goal.max_latency_ms}ms |
            可用性 &gt; {selectedScenario.goal.availability}%
          </div>
        </div>
      )}

      {hoveredTool && (
        <div
          className="tool-intro-card"
          style={{
            position: 'fixed',
            left: mousePos.x + 20,
            top: mousePos.y - 40,
            pointerEvents: 'none',
            zIndex: 9999
          }}
        >
          <h4>{hoveredTool.name}</h4>
          <p>{toolDescriptions[hoveredTool.type]}</p>
        </div>
      )}
    </div>
  );
}

export default function App() {
  return (
    <ReactFlowProvider>
      <Game />
    </ReactFlowProvider>
  );
}

import { useEffect, useState } from "react";
import {
  Activity,
  Boxes,
  Cpu,
  LayoutDashboard,
  Layers,
  Server,
  Settings,
  Thermometer,
  Zap,
} from "lucide-react";
import "./App.css";
import GPUHistoryChart from "./components/GPUHistoryChart";
import PoolManagement from "./pages/PoolManagement";
import GPUMonitor from "./pages/GPUMonitor";
type GPU = {
  index: number;
  name: string;
  utilization: number;
  temperature: number;
  power_w: number;
  memory_total_mb: number;
  memory_used_mb: number;
  memory_free_mb: number;
};

type SystemInfo = {
  cpu?: {
    model_name: string;
    logical_cores: number;
    usage_percent: number;
  };
  memory?: {
    total_bytes: number;
    used_bytes: number;
    available_bytes: number;
    used_percent: number;
  };
  disks?: {
    device: string;
    mount_point: string;
    file_system: string;
    total_bytes: number;
    used_bytes: number;
    free_bytes: number;
    used_percent: number;
  }[];
  networks?: {
    name: string;
    bytes_recv: number;
    bytes_sent: number;
  }[];
};

type Node = {
  node_id: string;
  hostname: string;
  status: string;
  collected_at?: string;
  last_heartbeat?: string;
  snapshot?: {
    gpus?: GPU[];
    system?: SystemInfo;
  };
};

type Pool = {
  pool_id: number;
  name: string;
  status: string;
};

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes === 0) return "0 B";

  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const index = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );

  return `${(bytes / 1024 ** index).toFixed(2)} ${units[index]}`;
}

function formatDateTime(value?: string): string {
  if (!value) return "—";

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString("zh-CN");
}

function App() {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [pools, setPools] = useState<Pool[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [currentPage, setCurrentPage] = useState("dashboard");
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [nodeSearch, setNodeSearch] = useState("");
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);
  const [nodeDetailError, setNodeDetailError] = useState("");

  useEffect(() => {
    let active = true;

    async function loadData() {
      try {
        const [nodesRes, poolsRes] = await Promise.all([
          fetch("/api/v1/nodes"),
          fetch("/api/v1/pools"),
        ]);

        if (!nodesRes.ok || !poolsRes.ok) {
          throw new Error("Coordinator API 请求失败");
        }

        const nodesData = await nodesRes.json();
        const poolsData = await poolsRes.json();

        if (active) {
          setNodes(nodesData.nodes ?? []);
          setPools(poolsData.pools ?? []);
          setError("");
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : "数据加载失败");
        }
      } finally {
        if (active) setLoading(false);
      }
    }

    void loadData();
    const timer = window.setInterval(() => void loadData(), 5000);

    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    if (!selectedNodeId || currentPage !== "nodes") return;

    const controller = new AbortController();

    async function loadNodeDetail() {
      try {
        const response = await fetch(
          `/api/v1/nodes/${encodeURIComponent(selectedNodeId!)}`,
          { signal: controller.signal },
        );

        if (!response.ok) {
          throw new Error(`节点详情请求失败：HTTP ${response.status}`);
        }

        const result: { node: Node } = await response.json();

        if (!controller.signal.aborted) {
          setSelectedNode(result.node);
          setNodeDetailError("");
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setNodeDetailError(
            err instanceof Error ? err.message : "节点详情加载失败",
          );
        }
      }
    }

    setSelectedNode(null);
    setNodeDetailError("");
    void loadNodeDetail();

    const timer = window.setInterval(() => {
      void loadNodeDetail();
    }, 5000);

    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [selectedNodeId, currentPage]);

  const onlineNodes = nodes.filter((node) => node.status === "online");
  const gpus = nodes.flatMap((node) => node.snapshot?.gpus ?? []);
  const totalMemory = gpus.reduce((sum, gpu) => sum + gpu.memory_total_mb, 0);
  const usedMemory = gpus.reduce((sum, gpu) => sum + gpu.memory_used_mb, 0);
  const gpu = gpus[0];

  const menu = [
    { icon: LayoutDashboard, label: "总览", key: "dashboard" },
    { icon: Server, label: "节点管理", key: "nodes" },
    { icon: Layers, label: "资源池", key: "pools" },
    { icon: Activity, label: "GPU 监控", key: "gpu" },
    { icon: Boxes, label: "模型管理", key: "models" },
    { icon: Cpu, label: "推理任务", key: "tasks" },
    { icon: Settings, label: "系统设置", key: "settings" },
  ];

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-icon">
            <Layers size={26} />
          </div>
          <div>
            <strong>GPU-Pool</strong>
            <span>GPU 算力管理平台</span>
          </div>
        </div>

        <nav className="navigation">
          {menu.map(({ icon: Icon, label, key }) => (
            <button
              key={key}
              type="button"
              className={`nav-item ${currentPage === key ? "active" : ""}`}
              onClick={() => setCurrentPage(key)}
            >
              <Icon size={19} />
              <span>{label}</span>
            </button>
          ))}
        </nav>

        <div className="sidebar-footer">GPU-Pool · Development</div>
      </aside>

      <main className="main-content">
        <header className="page-header">
          <div>
            <h1>{menu.find((item) => item.key === currentPage)?.label}</h1>
            <p>
              {currentPage === "dashboard"
                ? "GPU 集群资源概览与运行状态"
                : currentPage === "nodes"
                  ? "查看计算节点状态、GPU 资源及硬件详情"
                  : "GPU-Pool 算力管理平台"}
            </p>
          </div>
          <div className="header-status">
            <span className={`status-dot ${error ? "offline" : ""}`} />
            {error ? "Coordinator 连接异常" : "Coordinator 运行中"}
          </div>
        </header>

        {error && <div className="error-banner">{error}</div>}

        {currentPage === "dashboard" && (
          <>
            <section className="metrics-grid">
              <MetricCard
                title="在线节点数"
                value={`${onlineNodes.length} / ${nodes.length}`}
                icon={<Server size={25} />}
                detail="实时节点状态"
              />
              <MetricCard
                title="GPU 总数"
                value={gpus.length.toString()}
                icon={<Cpu size={25} />}
                detail={gpu?.name ?? "暂无 GPU"}
              />
              <MetricCard
                title="显存总量"
                value={`${totalMemory.toLocaleString()} MB`}
                icon={<Zap size={25} />}
                detail={`已用 ${usedMemory.toLocaleString()} MB`}
              />
              <MetricCard
                title="资源池数量"
                value={pools.length.toString()}
                icon={<Layers size={25} />}
                detail="已创建资源池"
              />
            </section>

            <section className="detail-grid">
              <div className="panel">
                <h2>GPU 资源使用情况</h2>

                {gpus.length > 0 ? (
                  <div className="gpu-panel-list">
                    {gpus.map((gpu, index) => (
                      <div className="gpu-panel" key={index}>
                        <div
                          className="utilization-circle"
                          style={{
                            background: `conic-gradient(#34df9a 0% ${gpu.utilization}%, #2b3952 ${gpu.utilization}% 100%)`,
                          }}
                        >
                          <strong>{gpu.utilization}%</strong>
                          <span>GPU 利用率</span>
                        </div>

                        <div className="gpu-details">
                          <div className="gpu-name">{gpu.name}</div>
                          <InfoRow
                            label="显存使用"
                            value={`${gpu.memory_used_mb} / ${gpu.memory_total_mb} MB`}
                          />
                          <div className="progress-track">
                            <div
                              className="progress-fill"
                              style={{
                                width: `${Math.min(
                                  100,
                                  (gpu.memory_used_mb /
                                    Math.max(gpu.memory_total_mb, 1)) *
                                    100,
                                )}%`,
                              }}
                            />
                          </div>
                          <InfoRow
                            label="GPU 温度"
                            value={`${gpu.temperature} °C`}
                            icon={<Thermometer size={16} />}
                          />
                          <InfoRow
                            label="GPU 功耗"
                            value={`${gpu.power_w} W`}
                            icon={<Zap size={16} />}
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="empty-state">
                    {loading ? "正在加载..." : "暂无 GPU 数据"}
                  </p>
                )}
              </div>

              <div className="panel">
                <h2>集群运行状态</h2>
                <div className="cluster-summary">
                  <InfoRow label="节点总数" value={nodes.length.toString()} />
                  <InfoRow
                    label="在线节点"
                    value={onlineNodes.length.toString()}
                  />
                  <InfoRow label="GPU 总数" value={gpus.length.toString()} />
                  <InfoRow label="资源池" value={pools.length.toString()} />
                  <InfoRow label="数据刷新" value="每 5 秒" />
                </div>
              </div>
            </section>
            {nodes[0]?.snapshot?.gpus?.[0] && (
              <GPUHistoryChart
                nodeId={nodes[0].node_id}
                gpuIndex={nodes[0].snapshot.gpus[0].index ?? 0}
              />
            )}

            <section className="detail-grid tables-grid">
              <div className="panel">
                <h2>节点列表</h2>
                <div className="table-wrapper">
                  <table>
                    <thead>
                      <tr>
                        <th>节点名称</th>
                        <th>状态</th>
                        <th>GPU 数</th>
                        <th>GPU 型号</th>
                      </tr>
                    </thead>
                    <tbody>
                      {nodes.map((node) => (
                        <tr key={node.node_id}>
                          <td>{node.hostname}</td>
                          <td>
                            <span className={`node-status ${node.status}`}>
                              {node.status}
                            </span>
                          </td>
                          <td>{node.snapshot?.gpus?.length ?? 0}</td>
                          <td>{node.snapshot?.gpus?.[0]?.name ?? "—"}</td>
                        </tr>
                      ))}
                      {nodes.length === 0 && (
                        <tr>
                          <td colSpan={4}>暂无节点</td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>

              <div className="panel">
                <h2>资源池列表</h2>
                <div className="table-wrapper">
                  <table>
                    <thead>
                      <tr>
                        <th>资源池</th>
                        <th>ID</th>
                        <th>状态</th>
                      </tr>
                    </thead>
                    <tbody>
                      {pools.map((pool) => (
                        <tr key={pool.pool_id}>
                          <td>{pool.name}</td>
                          <td>#{pool.pool_id}</td>
                          <td>{pool.status}</td>
                        </tr>
                      ))}
                      {pools.length === 0 && (
                        <tr>
                          <td colSpan={3}>暂无资源池</td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </section>
          </>
        )}

        {currentPage === "nodes" && (
          <section className="node-management">
            <div className="panel">
              <div className="node-toolbar">
                <div>
                  <h2>计算节点</h2>
                  <p>
                    共 {nodes.length} 台节点，在线 {onlineNodes.length} 台
                  </p>
                </div>

                <input
                  type="search"
                  placeholder="搜索节点名称或 ID"
                  value={nodeSearch}
                  onChange={(event) => setNodeSearch(event.target.value)}
                />
              </div>

              <div className="table-wrapper">
                <table>
                  <thead>
                    <tr>
                      <th>节点名称</th>
                      <th>状态</th>
                      <th>GPU 数量</th>
                      <th>GPU 型号</th>
                      <th>GPU 利用率</th>
                      <th>操作</th>
                    </tr>
                  </thead>

                  <tbody>
                    {nodes
                      .filter((node) =>
                        `${node.hostname} ${node.node_id}`
                          .toLowerCase()
                          .includes(nodeSearch.toLowerCase()),
                      )
                      .map((node) => (
                        <tr key={node.node_id}>
                          <td>{node.hostname}</td>
                          <td>
                            <span className={`node-status ${node.status}`}>
                              {node.status === "online" ? "在线" : node.status}
                            </span>
                          </td>
                          <td>{node.snapshot?.gpus?.length ?? 0}</td>
                          <td>{node.snapshot?.gpus?.[0]?.name ?? "—"}</td>
                          <td>
                            {node.snapshot?.gpus?.[0]
                              ? `${node.snapshot.gpus[0].utilization}%`
                              : "—"}
                          </td>
                          <td>
                            <button
                              type="button"
                              className="node-detail-button"
                              onClick={() => setSelectedNodeId(node.node_id)}
                            >
                              查看详情
                            </button>
                          </td>
                        </tr>
                      ))}

                    {nodes.filter((node) =>
                      `${node.hostname} ${node.node_id}`
                        .toLowerCase()
                        .includes(nodeSearch.toLowerCase()),
                    ).length === 0 && (
                      <tr>
                        <td colSpan={6}>没有找到符合条件的节点</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            {selectedNodeId && (
              <div className="panel node-detail-panel">
                <div className="node-toolbar">
                  <div>
                    <h2>节点详情：{selectedNode?.hostname ?? "加载中..."}</h2>
                    <p>完整硬件资源与运行状态</p>
                  </div>

                  <button
                    type="button"
                    className="node-detail-button"
                    onClick={() => setSelectedNodeId(null)}
                  >
                    关闭
                  </button>
                </div>

                {nodeDetailError && (
                  <p className="history-error">{nodeDetailError}</p>
                )}

                {!selectedNode && !nodeDetailError && (
                  <p className="empty-state">正在加载节点详情...</p>
                )}

                {selectedNode && (
                  <>
                    <div className="node-info-grid">
                      <div className="node-info-card">
                        <h3>节点基本信息</h3>
                        <InfoRow
                          label="主机名称"
                          value={selectedNode.hostname}
                        />
                        <InfoRow label="节点 ID" value={selectedNode.node_id} />
                        <InfoRow label="运行状态" value={selectedNode.status} />
                        <InfoRow
                          label="最后心跳"
                          value={formatDateTime(selectedNode.last_heartbeat)}
                        />
                        <InfoRow
                          label="最后采集"
                          value={formatDateTime(selectedNode.collected_at)}
                        />
                      </div>

                      <div className="node-info-card">
                        <h3>CPU</h3>
                        <InfoRow
                          label="CPU 型号"
                          value={
                            selectedNode.snapshot?.system?.cpu?.model_name ??
                            "—"
                          }
                        />
                        <InfoRow
                          label="逻辑核心数"
                          value={String(
                            selectedNode.snapshot?.system?.cpu?.logical_cores ??
                              "—",
                          )}
                        />
                        <InfoRow
                          label="CPU 使用率"
                          value={
                            selectedNode.snapshot?.system?.cpu
                              ? `${selectedNode.snapshot.system.cpu.usage_percent.toFixed(1)}%`
                              : "—"
                          }
                        />
                      </div>

                      <div className="node-info-card">
                        <h3>系统内存</h3>
                        <InfoRow
                          label="总内存"
                          value={
                            selectedNode.snapshot?.system?.memory
                              ? formatBytes(
                                  selectedNode.snapshot.system.memory
                                    .total_bytes,
                                )
                              : "—"
                          }
                        />
                        <InfoRow
                          label="已使用"
                          value={
                            selectedNode.snapshot?.system?.memory
                              ? formatBytes(
                                  selectedNode.snapshot.system.memory
                                    .used_bytes,
                                )
                              : "—"
                          }
                        />
                        <InfoRow
                          label="可用内存"
                          value={
                            selectedNode.snapshot?.system?.memory
                              ? formatBytes(
                                  selectedNode.snapshot.system.memory
                                    .available_bytes,
                                )
                              : "—"
                          }
                        />
                        <InfoRow
                          label="内存使用率"
                          value={
                            selectedNode.snapshot?.system?.memory
                              ? `${selectedNode.snapshot.system.memory.used_percent}%`
                              : "—"
                          }
                        />
                      </div>

                      <div className="node-info-card">
                        <h3>GPU</h3>
                        {(selectedNode.snapshot?.gpus ?? []).map(
                          (gpu, index) => (
                            <div key={gpu.index ?? index}>
                              <InfoRow label="GPU 型号" value={gpu.name} />
                              <InfoRow
                                label="GPU 利用率"
                                value={`${gpu.utilization}%`}
                              />
                              <InfoRow
                                label="显存"
                                value={`${gpu.memory_used_mb} / ${gpu.memory_total_mb} MB`}
                              />
                              <InfoRow
                                label="温度"
                                value={`${gpu.temperature} °C`}
                              />
                              <InfoRow
                                label="功耗"
                                value={`${gpu.power_w} W`}
                              />
                            </div>
                          ),
                        )}
                      </div>
                    </div>
                  </>
                )}
              </div>
            )}
          </section>
        )}

        {currentPage === "pools" && <PoolManagement />}
        {currentPage === "gpu" && <GPUMonitor nodes={nodes} />}

        {currentPage !== "dashboard" &&
          currentPage !== "nodes" &&
          currentPage !== "pools" &&
          currentPage !== "gpu" && (
            <div className="panel">
              <h2>{menu.find((item) => item.key === currentPage)?.label}</h2>
              <p className="empty-state">该模块将在后续阶段开发。</p>
            </div>
          )}
      </main>
    </div>
  );
}

function MetricCard({
  title,
  value,
  icon,
  detail,
}: {
  title: string;
  value: string;
  icon: React.ReactNode;
  detail: string;
}) {
  return (
    <div className="metric-card">
      <div className="metric-top">
        <span>{title}</span>
        <div className="metric-icon">{icon}</div>
      </div>
      <strong>{value}</strong>
      <small>{detail}</small>
    </div>
  );
}

function InfoRow({
  label,
  value,
  icon,
}: {
  label: string;
  value: string;
  icon?: React.ReactNode;
}) {
  return (
    <div className="info-row">
      <span>
        {icon}
        {label}
      </span>
      <strong>{value}</strong>
    </div>
  );
}

export default App;

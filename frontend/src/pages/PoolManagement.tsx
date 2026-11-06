import { useCallback, useEffect, useState } from "react";

type Pool = {
  pool_id: number;
  name: string;
  description?: string;
  status: string;
  node_count: number;
  created_at: string;
};

type PoolStats = {
  pool_id: number;
  node_count: number;
  gpu_count: number;
  memory_total_mb: number;
  memory_used_mb: number;
  memory_free_mb: number;
};

type PoolNode = {
  pool_id: number;
  node_id: string;
  hostname: string;
  joined_at: string;
};

type PoolListResponse = {
  pools: Pool[];
  total: number;
};

type PoolNodesResponse = {
  nodes: PoolNode[];
  total: number;
};

function formatDate(value?: string) {
  if (!value) return "—";

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString("zh-CN");
}

export default function PoolManagement() {
  const [pools, setPools] = useState<Pool[]>([]);
  const [selectedPoolId, setSelectedPoolId] = useState<number | null>(null);
  const [stats, setStats] = useState<PoolStats | null>(null);
  const [poolNodes, setPoolNodes] = useState<PoolNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState("");
  const [detailError, setDetailError] = useState("");
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newPoolName, setNewPoolName] = useState("");
  const [newPoolDescription, setNewPoolDescription] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState("");
  const [actionMessage, setActionMessage] = useState("");

  const [availableNodes, setAvailableNodes] = useState<
    { node_id: string; hostname: string; status: string }[]
  >([]);

  const [nodeToAdd, setNodeToAdd] = useState("");

  const [editingPoolId, setEditingPoolId] = useState<number | null>(null);
  const [editPoolName, setEditPoolName] = useState("");
  const [editPoolDescription, setEditPoolDescription] = useState("");
  const [poolActionLoading, setPoolActionLoading] = useState(false);

  const loadPools = useCallback(async (signal?: AbortSignal) => {
    try {
      const response = await fetch("/api/v1/pools", { signal });

      if (!response.ok) {
        throw new Error(`资源池列表请求失败：HTTP ${response.status}`);
      }

      const result: PoolListResponse = await response.json();

      if (!signal?.aborted) {
        setPools(result.pools ?? []);
        setError("");
      }
    } catch (err) {
      if (!signal?.aborted) {
        setError(err instanceof Error ? err.message : "资源池加载失败");
      }
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();

    void loadPools(controller.signal);

    const timer = window.setInterval(() => {
      void loadPools(controller.signal);
    }, 10000);

    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [loadPools]);

  useEffect(() => {
    if (selectedPoolId === null) return;

    const controller = new AbortController();

    async function loadDetails() {
      try {
        const [statsResponse, nodesResponse] = await Promise.all([
          fetch(`/api/v1/pools/${selectedPoolId}/stats`, {
            signal: controller.signal,
          }),
          fetch(`/api/v1/pools/${selectedPoolId}/nodes`, {
            signal: controller.signal,
          }),
        ]);

        if (!statsResponse.ok || !nodesResponse.ok) {
          throw new Error("资源池详情查询失败");
        }

        const statsResult: PoolStats = await statsResponse.json();
        const nodesResult: PoolNodesResponse = await nodesResponse.json();

        if (!controller.signal.aborted) {
          setStats(statsResult);
          setPoolNodes(nodesResult.nodes ?? []);
          setDetailError("");
          setDetailLoading(false);
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setDetailError(
            err instanceof Error ? err.message : "资源池详情加载失败",
          );
          setDetailLoading(false);
        }
      }
    }

    setStats(null);
    setPoolNodes([]);
    setDetailError("");
    setDetailLoading(true);

    void loadDetails();

    const timer = window.setInterval(() => {
      void loadDetails();
    }, 10000);

    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [selectedPoolId]);

  async function createPool() {
    const name = newPoolName.trim();

    if (!name) {
      setActionError("请输入资源池名称");
      return;
    }

    setSubmitting(true);
    setActionError("");
    setActionMessage("");

    try {
      const response = await fetch("/api/v1/pools", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name,
          description: newPoolDescription.trim(),
        }),
      });

      if (!response.ok) {
        const body = await response.text();
        throw new Error(`创建失败：HTTP ${response.status} ${body}`);
      }

      setShowCreateForm(false);
      setNewPoolName("");
      setNewPoolDescription("");
      setActionMessage(`资源池 ${name} 创建成功`);

      await loadPools();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "创建资源池失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function loadAvailableNodes() {
    try {
      const [nodesResponse, poolsResponse] = await Promise.all([
        fetch("/api/v1/nodes"),
        fetch("/api/v1/pools"),
      ]);

      if (!nodesResponse.ok || !poolsResponse.ok) {
        throw new Error("查询可用节点失败");
      }

      const nodesResult: {
        nodes: {
          node_id: string;
          hostname: string;
          status: string;
        }[];
      } = await nodesResponse.json();

      const poolsResult: PoolListResponse = await poolsResponse.json();

      const poolList = poolsResult.pools ?? [];

      const memberResults = await Promise.all(
        poolList.map(async (pool) => {
          const response = await fetch(`/api/v1/pools/${pool.pool_id}/nodes`);

          if (!response.ok) {
            throw new Error(`查询资源池 ${pool.name} 成员失败`);
          }

          const result: PoolNodesResponse = await response.json();
          return result.nodes ?? [];
        }),
      );

      const assignedNodeIds = new Set(
        memberResults.flat().map((node) => node.node_id),
      );

      const available = (nodesResult.nodes ?? []).filter(
        (node) =>
          node.status === "online" && !assignedNodeIds.has(node.node_id),
      );

      setAvailableNodes(available);
      setNodeToAdd(available[0]?.node_id ?? "");
      setActionError("");
    } catch (err) {
      setAvailableNodes([]);
      setNodeToAdd("");
      setActionError(err instanceof Error ? err.message : "加载可用节点失败");
    }
  }

  async function addNodeToPool() {
    if (selectedPoolId === null || !nodeToAdd) {
      setActionError("请选择需要加入的节点");
      return;
    }

    setSubmitting(true);
    setActionError("");
    setActionMessage("");

    try {
      const response = await fetch(`/api/v1/pools/${selectedPoolId}/nodes`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          node_id: nodeToAdd,
        }),
      });

      if (!response.ok) {
        const body = await response.text();
        throw new Error(`添加节点失败：HTTP ${response.status} ${body}`);
      }

      setActionMessage("节点添加成功");
      setNodeToAdd("");

      await Promise.all([loadPools(), loadAvailableNodes()]);

      // 详情数据由现有的定时刷新逻辑更新
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "添加节点失败");
    } finally {
      setSubmitting(false);
    }
  }

  function startEditPool(pool: Pool) {
    setEditingPoolId(pool.pool_id);
    setEditPoolName(pool.name);
    setEditPoolDescription(pool.description ?? "");
    setActionError("");
    setActionMessage("");
    setShowCreateForm(false);
    setSelectedPoolId(null);
  }

  async function savePoolEdit() {
    if (editingPoolId === null) return;

    const name = editPoolName.trim();

    if (!name) {
      setActionError("资源池名称不能为空");
      return;
    }

    setPoolActionLoading(true);
    setActionError("");
    setActionMessage("");

    try {
      const response = await fetch(`/api/v1/pools/${editingPoolId}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name,
          description: editPoolDescription.trim(),
        }),
      });

      if (!response.ok) {
        throw new Error(`修改资源池失败：HTTP ${response.status}`);
      }

      setEditingPoolId(null);
      setActionMessage(`资源池 ${name} 修改成功`);
      await loadPools();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "修改资源池失败");
    } finally {
      setPoolActionLoading(false);
    }
  }

  async function changePoolStatus(pool: Pool) {
    const nextStatus = pool.status === "active" ? "disabled" : "active";
    const actionName = nextStatus === "active" ? "启用" : "禁用";

    if (!window.confirm(`确定${actionName}资源池 ${pool.name} 吗？`)) {
      return;
    }

    setPoolActionLoading(true);
    setActionError("");
    setActionMessage("");

    try {
      const response = await fetch(`/api/v1/pools/${pool.pool_id}/status`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          status: nextStatus,
        }),
      });

      if (!response.ok) {
        throw new Error(`${actionName}失败：HTTP ${response.status}`);
      }

      setActionMessage(`资源池 ${pool.name} 已${actionName}`);
      await loadPools();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "修改资源池状态失败");
    } finally {
      setPoolActionLoading(false);
    }
  }

  async function deletePool(pool: Pool) {
    if (pool.node_count > 0) {
      setActionError("请先移除资源池中的所有成员节点");
      return;
    }

    if (
      !window.confirm(`确定永久删除资源池 ${pool.name} 吗？此操作不可撤销。`)
    ) {
      return;
    }

    setPoolActionLoading(true);
    setActionError("");
    setActionMessage("");

    try {
      const response = await fetch(`/api/v1/pools/${pool.pool_id}`, {
        method: "DELETE",
      });

      if (!response.ok) {
        throw new Error(`删除资源池失败：HTTP ${response.status}`);
      }

      if (selectedPoolId === pool.pool_id) {
        setSelectedPoolId(null);
      }

      setActionMessage(`资源池 ${pool.name} 已删除`);
      await loadPools();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "删除资源池失败");
    } finally {
      setPoolActionLoading(false);
    }
  }

  async function removePoolNode(node: PoolNode) {
    if (selectedPoolId === null) return;

    if (!window.confirm(`确定将节点 ${node.hostname} 从当前资源池移除吗？`)) {
      return;
    }

    setPoolActionLoading(true);
    setActionError("");
    setActionMessage("");

    try {
      const response = await fetch(
        `/api/v1/pools/${selectedPoolId}/nodes/${encodeURIComponent(node.node_id)}`,
        {
          method: "DELETE",
        },
      );

      if (!response.ok) {
        throw new Error(`移除节点失败：HTTP ${response.status}`);
      }

      setPoolNodes((previous) =>
        previous.filter((item) => item.node_id !== node.node_id),
      );

      setStats((previous) =>
        previous
          ? {
              ...previous,
              node_count: Math.max(0, previous.node_count - 1),
            }
          : null,
      );

      setActionMessage(`节点 ${node.hostname} 已移出资源池`);

      await Promise.all([loadPools(), loadAvailableNodes()]);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "移除节点失败");
    } finally {
      setPoolActionLoading(false);
    }
  }

  const selectedPool = pools.find((pool) => pool.pool_id === selectedPoolId);

  return (
    <div className="pool-management">
      <div className="panel">
        <div className="pool-header">
          <div>
            <h2>资源池管理</h2>
            <p>统一管理 GPU 算力资源池与成员节点</p>
          </div>

          <div className="pool-actions">
            <button
              type="button"
              className="pool-refresh-button"
              onClick={() => {
                setShowCreateForm((previous) => !previous);
                setSelectedPoolId(null);
              }}
            >
              {showCreateForm ? "取消创建" : "创建资源池"}
            </button>

            <button
              type="button"
              className="pool-refresh-button"
              onClick={() => void loadPools()}
            >
              刷新列表
            </button>
          </div>
        </div>
        {showCreateForm && (
          <form
            className="pool-create-form"
            onSubmit={(event) => {
              event.preventDefault();
              void createPool();
            }}
          >
            <h3>创建新资源池</h3>

            <label htmlFor="new-pool-name">资源池名称</label>
            <input
              id="new-pool-name"
              value={newPoolName}
              onChange={(event) => setNewPoolName(event.target.value)}
              placeholder="例如：gpu-inference-pool"
              required
            />

            <label htmlFor="new-pool-description">资源池描述</label>
            <textarea
              id="new-pool-description"
              value={newPoolDescription}
              onChange={(event) => setNewPoolDescription(event.target.value)}
              placeholder="描述资源池用途"
              rows={3}
            />

            <button
              type="submit"
              className="pool-refresh-button"
              disabled={submitting}
            >
              {submitting ? "创建中..." : "确认创建"}
            </button>
          </form>
        )}

        {editingPoolId !== null && (
          <form
            className="pool-create-form"
            onSubmit={(event) => {
              event.preventDefault();
              void savePoolEdit();
            }}
          >
            <h3>编辑资源池 #{editingPoolId}</h3>

            <label htmlFor="edit-pool-name">资源池名称</label>
            <input
              id="edit-pool-name"
              value={editPoolName}
              onChange={(event) => setEditPoolName(event.target.value)}
              required
            />

            <label htmlFor="edit-pool-description">资源池描述</label>
            <textarea
              id="edit-pool-description"
              value={editPoolDescription}
              onChange={(event) => setEditPoolDescription(event.target.value)}
              rows={3}
            />

            <div className="pool-actions">
              <button
                type="submit"
                className="pool-refresh-button"
                disabled={poolActionLoading}
              >
                保存修改
              </button>

              <button
                type="button"
                className="pool-refresh-button"
                onClick={() => setEditingPoolId(null)}
              >
                取消
              </button>
            </div>
          </form>
        )}

        <div className="pool-overview">
          <div className="pool-stat-card">
            <span>资源池总数</span>
            <strong>{pools.length}</strong>
          </div>

          <div className="pool-stat-card">
            <span>活跃资源池</span>
            <strong>
              {pools.filter((pool) => pool.status === "active").length}
            </strong>
          </div>

          <div className="pool-stat-card">
            <span>资源池成员数</span>
            <strong>
              {pools.reduce((sum, pool) => sum + pool.node_count, 0)}
            </strong>
          </div>
        </div>

        {error && <p className="history-error">{error}</p>}

        <div className="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>资源池名称</th>
                <th>资源池 ID</th>
                <th>状态</th>
                <th>节点数</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>

            <tbody>
              {pools.map((pool) => (
                <tr key={pool.pool_id}>
                  <td>{pool.name}</td>
                  <td>#{pool.pool_id}</td>
                  <td>
                    <span
                      className={
                        pool.status === "active"
                          ? "pool-status-active"
                          : "pool-status-disabled"
                      }
                    >
                      {pool.status === "active" ? "运行中" : pool.status}
                    </span>
                  </td>
                  <td>{pool.node_count}</td>
                  <td>{formatDate(pool.created_at)}</td>
                  <td>
                    <div className="pool-table-actions">
                      <button
                        type="button"
                        className="node-detail-button"
                        onClick={() => {
                          setSelectedPoolId(pool.pool_id);
                          setShowCreateForm(false);
                          setEditingPoolId(null);
                        }}
                      >
                        查看详情
                      </button>

                      <button
                        type="button"
                        className="node-detail-button"
                        disabled={poolActionLoading}
                        onClick={() => startEditPool(pool)}
                      >
                        编辑
                      </button>

                      <button
                        type="button"
                        className="node-detail-button"
                        disabled={poolActionLoading}
                        onClick={() => void changePoolStatus(pool)}
                      >
                        {pool.status === "active" ? "禁用" : "启用"}
                      </button>

                      <button
                        type="button"
                        className="pool-danger-button"
                        disabled={poolActionLoading || pool.node_count > 0}
                        onClick={() => void deletePool(pool)}
                        title={
                          pool.node_count > 0
                            ? "请先移除成员节点"
                            : "删除资源池"
                        }
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}

              {!loading && pools.length === 0 && (
                <tr>
                  <td colSpan={6}>暂无资源池</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {loading && <p className="empty-state">正在加载资源池...</p>}
      </div>

      {selectedPoolId !== null && (
        <div className="panel pool-detail-panel">
          <div className="pool-header">
            <div>
              <h2>资源池详情：{selectedPool?.name ?? `#${selectedPoolId}`}</h2>
              <p>资源统计与成员节点</p>
            </div>

            <button
              type="button"
              className="node-detail-button"
              onClick={() => setSelectedPoolId(null)}
            >
              关闭
            </button>
          </div>

          {actionError && <p className="history-error">{actionError}</p>}
          {actionMessage && (
            <p className="pool-success-message">{actionMessage}</p>
          )}

          {detailError && <p className="history-error">{detailError}</p>}

          {detailLoading && (
            <p className="empty-state">正在加载资源池详情...</p>
          )}

          {stats && (
            <>
              <div className="pool-overview">
                <div className="pool-stat-card">
                  <span>成员节点</span>
                  <strong>{stats.node_count}</strong>
                </div>

                <div className="pool-stat-card">
                  <span>GPU 数量</span>
                  <strong>{stats.gpu_count}</strong>
                </div>

                <div className="pool-stat-card">
                  <span>显存总量</span>
                  <strong>{stats.memory_total_mb} MB</strong>
                </div>

                <div className="pool-stat-card">
                  <span>已用显存</span>
                  <strong>{stats.memory_used_mb} MB</strong>
                </div>

                <div className="pool-stat-card">
                  <span>剩余显存</span>
                  <strong>{stats.memory_free_mb} MB</strong>
                </div>
              </div>

              <h3 className="pool-section-title">成员节点</h3>

              <div className="pool-add-node">
                <select
                  value={nodeToAdd}
                  onChange={(event) => setNodeToAdd(event.target.value)}
                >
                  <option value="">请选择可用节点</option>
                  {availableNodes.map((node) => (
                    <option key={node.node_id} value={node.node_id}>
                      {node.hostname} ({node.node_id.slice(0, 8)})
                    </option>
                  ))}
                </select>

                <button
                  type="button"
                  className="pool-refresh-button"
                  onClick={() => void loadAvailableNodes()}
                >
                  查询可用节点
                </button>

                <button
                  type="button"
                  className="pool-refresh-button"
                  disabled={submitting || !nodeToAdd}
                  onClick={() => void addNodeToPool()}
                >
                  {submitting ? "添加中..." : "添加节点"}
                </button>
              </div>

              <div className="table-wrapper">
                <table>
                  <thead>
                    <tr>
                      <th>节点名称</th>
                      <th>节点 ID</th>
                      <th>加入时间</th>
                      <th>操作</th>
                    </tr>
                  </thead>

                  <tbody>
                    {poolNodes.map((node) => (
                      <tr key={node.node_id}>
                        <td>{node.hostname}</td>
                        <td>{node.node_id}</td>
                        <td>{formatDate(node.joined_at)}</td>
                        <td>
                          <button
                            type="button"
                            className="pool-danger-button"
                            disabled={poolActionLoading}
                            onClick={() => void removePoolNode(node)}
                          >
                            移除
                          </button>
                        </td>
                      </tr>
                    ))}

                    {poolNodes.length === 0 && (
                      <tr>
                        <td colSpan={4}>当前资源池暂无成员节点</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}

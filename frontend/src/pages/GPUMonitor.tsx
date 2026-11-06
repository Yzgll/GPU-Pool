import { Fragment, useMemo, useState } from "react";
import GPUHistoryChart from "../components/GPUHistoryChart";
import GPUClusterHistoryChart from "../components/GPUClusterHistoryChart";
import GPUAllDevicesHistoryChart from "../components/GPUAllDevicesHistoryChart";
type GPU = {
  index?: number;
  name: string;
  utilization: number;
  temperature: number;
  power_w: number;
  memory_total_mb: number;
  memory_used_mb: number;
  memory_free_mb: number;
};
type GPUDevice = {
  key: string;
  nodeId: string;
  hostname: string;
  nodeStatus: string;
  gpuIndex: number;
  gpu: GPU;
};
type Node = {
  node_id: string;
  hostname: string;
  status: string;
  snapshot?: {
    gpus?: GPU[];
  };
};

type Props = {
  nodes: Node[];
};

export default function GPUMonitor({ nodes }: Props) {
  const [expandedGPUKeys, setExpandedGPUKeys] = useState<string[]>([]);

  const toggleGPU = (key: string) => {
    setExpandedGPUKeys((current) =>
      current.includes(key)
        ? current.filter((item) => item !== key)
        : [...current, key],
    );
  };
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");

  const devices = useMemo<GPUDevice[]>(
    () =>
      nodes.flatMap((node) =>
        (node.snapshot?.gpus ?? []).map((gpu, position) => {
          const gpuIndex = gpu.index ?? position;

          return {
            key: `${node.node_id}:${gpuIndex}`,
            nodeId: node.node_id,
            hostname: node.hostname,
            nodeStatus: node.status,
            gpuIndex,
            gpu,
          };
        }),
      ),
    [nodes],
  );

  const filteredDevices = devices.filter((device) => {
    const keyword = search.trim().toLowerCase();

    const matchesSearch =
      !keyword ||
      device.hostname.toLowerCase().includes(keyword) ||
      device.gpu.name.toLowerCase().includes(keyword) ||
      String(device.gpuIndex).includes(keyword);

    const matchesStatus =
      statusFilter === "all" || device.nodeStatus === statusFilter;

    return matchesSearch && matchesStatus;
  });

  const onlineDevices = devices.filter(
    (device) => device.nodeStatus === "online",
  );

  const totalMemory = devices.reduce(
    (sum, device) => sum + device.gpu.memory_total_mb,
    0,
  );

  const averageUtilization =
    onlineDevices.length > 0
      ? onlineDevices.reduce((sum, device) => sum + device.gpu.utilization, 0) /
        onlineDevices.length
      : 0;

  return (
    <section className="gpu-monitor">
      <div className="panel">
        <div className="node-toolbar">
          <div>
            <h2>GPU 设备监控</h2>
            <p>统一查看各节点 GPU 资源与运行状态</p>
          </div>

          <span className="gpu-monitor-refresh">数据每 5 秒刷新</span>
        </div>

        <div className="gpu-monitor-summary">
          <div className="gpu-monitor-stat">
            <span>GPU 总数</span>
            <strong>{devices.length}</strong>
          </div>

          <div className="gpu-monitor-stat">
            <span>在线 GPU</span>
            <strong>{onlineDevices.length}</strong>
          </div>

          <div className="gpu-monitor-stat">
            <span>显存总量</span>
            <strong>{totalMemory.toLocaleString()} MB</strong>
          </div>

          <div className="gpu-monitor-stat">
            <span>平均 GPU 利用率</span>
            <strong>{averageUtilization.toFixed(1)}%</strong>
          </div>
        </div>

        <div className="gpu-monitor-filters">
          <input
            type="text"
            placeholder="搜索节点名称或 GPU 型号"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />

          <select
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value)}
          >
            <option value="all">全部状态</option>
            <option value="online">在线</option>
            <option value="offline">离线</option>
          </select>
        </div>

        <div className="table-wrapper gpu-monitor-table-scroll">
          <table>
            <thead>
              <tr>
                <th>节点名称</th>
                <th>GPU</th>
                <th>GPU 型号</th>
                <th>状态</th>
                <th>利用率</th>
                <th>显存使用</th>
                <th>温度</th>
                <th>功耗</th>
                <th>操作</th>
              </tr>
            </thead>

            <tbody>
              {filteredDevices.map((device) => {
                const expanded = expandedGPUKeys.includes(device.key);

                return (
                  <Fragment key={device.key}>
                    <tr>
                      <td>{device.hostname}</td>
                      <td>GPU {device.gpuIndex}</td>
                      <td>{device.gpu.name}</td>
                      <td>
                        <span
                          className={
                            device.nodeStatus === "online"
                              ? "gpu-monitor-online"
                              : "gpu-monitor-offline"
                          }
                        >
                          {device.nodeStatus === "online" ? "在线" : "离线"}
                        </span>
                      </td>
                      <td>
                        {device.nodeStatus === "online"
                          ? `${device.gpu.utilization}%`
                          : "—"}
                      </td>
                      <td>
                        {device.nodeStatus === "online"
                          ? `${device.gpu.memory_used_mb} / ${device.gpu.memory_total_mb} MB`
                          : "—"}
                      </td>
                      <td>
                        {device.nodeStatus === "online"
                          ? `${device.gpu.temperature} °C`
                          : "—"}
                      </td>
                      <td>
                        {device.nodeStatus === "online"
                          ? `${device.gpu.power_w} W`
                          : "—"}
                      </td>
                      <td>
                        <button
                          type="button"
                          className="node-detail-button"
                          onClick={() => toggleGPU(device.key)}
                        >
                          {expanded ? "收起详情" : "查看详情"}
                        </button>
                      </td>
                    </tr>

                    {expanded && (
                      <tr className="gpu-expanded-row">
                        <td colSpan={9}>
                          <div className="gpu-expanded-content">
                            <div className="node-toolbar">
                              <div>
                                <h3>
                                  {device.hostname} / GPU {device.gpuIndex}
                                </h3>
                                <p>{device.gpu.name}</p>
                              </div>
                            </div>

                            <div className="gpu-monitor-summary">
                              <div className="gpu-monitor-stat">
                                <span>GPU 利用率</span>
                                <strong>{device.gpu.utilization}%</strong>
                              </div>

                              <div className="gpu-monitor-stat">
                                <span>显存使用</span>
                                <strong>
                                  {device.gpu.memory_used_mb} /{" "}
                                  {device.gpu.memory_total_mb} MB
                                </strong>
                              </div>

                              <div className="gpu-monitor-stat">
                                <span>GPU 温度</span>
                                <strong>{device.gpu.temperature} °C</strong>
                              </div>

                              <div className="gpu-monitor-stat">
                                <span>GPU 功耗</span>
                                <strong>{device.gpu.power_w} W</strong>
                              </div>
                            </div>

                            <GPUHistoryChart
                              nodeId={device.nodeId}
                              gpuIndex={device.gpuIndex}
                            />
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}

              {filteredDevices.length === 0 && (
                <tr>
                  <td colSpan={9}>暂无符合条件的 GPU 设备</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <GPUClusterHistoryChart />
      <GPUAllDevicesHistoryChart />
    </section>
  );
}


import GPUHistoryChart from "../components/GPUHistoryChart";

export type GPUDevice = {
  key: string;
  nodeId: string;
  hostname: string;
  nodeStatus: string;
  gpuIndex: number;
  gpu: {
    name: string;
    utilization: number;
    temperature: number;
    power_w: number;
    memory_total_mb: number;
    memory_used_mb: number;
    memory_free_mb: number;
    memory_type?: string;
  };
};

type Props = {
  device: GPUDevice;
  onBack: () => void;
};

export default function GPUDetail({ device, onBack }: Props) {
  const { gpu } = device;

  return (
    <div className="gpu-detail-page">
      <button
        type="button"
        className="node-detail-button"
        onClick={onBack}
      >
        ← 返回 GPU 监控
      </button>

      <div className="panel">
        <div className="node-toolbar">
          <div>
            <h2>
              {device.hostname} / GPU {device.gpuIndex}
            </h2>
            <p>{gpu.name}</p>
          </div>

          <span
            className={
              device.nodeStatus === "online"
                ? "gpu-monitor-online"
                : "gpu-monitor-offline"
            }
          >
            {device.nodeStatus === "online" ? "在线" : "离线"}
          </span>
        </div>

        <div className="gpu-monitor-summary">
          <div className="gpu-monitor-stat">
            <span>GPU 利用率</span>
            <strong>{gpu.utilization}%</strong>
          </div>

          <div className="gpu-monitor-stat">
            <span>显存使用</span>
            <strong>
              {gpu.memory_used_mb} / {gpu.memory_total_mb} MB
            </strong>
          </div>

          <div className="gpu-monitor-stat">
            <span>GPU 温度</span>
            <strong>
              {gpu.memory_type === "unified"
                ? "—"
                : `${gpu.temperature} °C`}
            </strong>
          </div>

          <div className="gpu-monitor-stat">
            <span>GPU 功耗</span>
            <strong>{gpu.power_w} W</strong>
          </div>
        </div>
      </div>

      <GPUHistoryChart
        key={device.key}
        nodeId={device.nodeId}
        gpuIndex={device.gpuIndex}
      />
    </div>
  );
}

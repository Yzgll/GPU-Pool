
import { useEffect, useMemo, useState } from "react";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

type TimeRange = "1h" | "6h" | "24h";

type GPUDevice = {
  key: string;
  hostname: string;
  gpuIndex: number;
  gpuName: string;
};

type ComparePoint = {
  collected_at: string;
  node_id: string;
  gpu_index: number;
  utilization: number;
  memory_used_mb: number;
  temperature: number;
};

type CompareResponse = {
  range: string;
  interval_seconds: number;
  points: ComparePoint[];
};

type NodesResponse = {
  nodes: {
    node_id: string;
    hostname: string;
    snapshot?: {
      gpus?: {
        index?: number;
        name: string;
      }[];
    };
  }[];
};

type Metric = "utilization" | "memory" | "temperature";

const colors = [
  "#38bdf8",
  "#34d399",
  "#a78bfa",
  "#fbbf24",
  "#fb7185",
  "#22d3ee",
  "#f97316",
  "#818cf8",
  "#e879f9",
  "#84cc16",
  "#f472b6",
  "#2dd4bf",
];

const metrics: {
  key: Metric;
  title: string;
  unit: string;
}[] = [
  {
    key: "utilization",
    title: "GPU 利用率",
    unit: "%",
  },
  {
    key: "memory",
    title: "GPU 显存使用量",
    unit: " MB",
  },
  {
    key: "temperature",
    title: "GPU 温度",
    unit: " °C",
  },
];

function formatTime(value: number) {
  return new Date(value).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function GPUAllDevicesHistoryChart() {
  const [range, setRange] = useState<TimeRange>("1h");
  const [devices, setDevices] = useState<GPUDevice[]>([]);
  const [points, setPoints] = useState<ComparePoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();

    async function loadDevices() {
      try {
        const response = await fetch("/api/v1/nodes", {
          signal: controller.signal,
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const data: NodesResponse = await response.json();

        const allDevices = (data.nodes ?? []).flatMap((node) =>
          (node.snapshot?.gpus ?? []).map((gpu, position) => {
            const gpuIndex = gpu.index ?? position;

            return {
              key: `${node.node_id}:${gpuIndex}`,
              hostname: node.hostname,
              gpuIndex,
              gpuName: gpu.name,
            };
          }),
        );

        if (!controller.signal.aborted) {
          setDevices(allDevices);
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(
            err instanceof Error ? err.message : "获取 GPU 列表失败",
          );
        }
      }
    }

    loadDevices();

    const timer = window.setInterval(loadDevices, 30000);

    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, []);

  const deviceKey = devices.map((gpu) => gpu.key).join("|");

  useEffect(() => {
    const controller = new AbortController();

    async function loadHistory() {
      if (devices.length === 0) {
        setPoints([]);
        setLoading(false);
        return;
      }

      try {
        const params = new URLSearchParams({ range });

        devices.forEach((gpu) => {
          params.append("gpu", gpu.key);
        });

        const response = await fetch(
          `/api/v1/gpus/history/compare?${params.toString()}`,
          { signal: controller.signal },
        );

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const data: CompareResponse = await response.json();

        if (!controller.signal.aborted) {
          setPoints(data.points ?? []);
          setError("");
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(
            err instanceof Error ? err.message : "获取历史数据失败",
          );
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    }

    setLoading(true);
    setPoints([]);
    loadHistory();

    const timer = window.setInterval(loadHistory, 10000);

    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [range, deviceKey]);

  const chartData = useMemo(() => {
    const buckets = new Map<
      number,
      Record<string, number>
    >();

    for (const point of points) {
      const time = new Date(point.collected_at).getTime();
      const key = `${point.node_id}:${point.gpu_index}`;

      if (!buckets.has(time)) {
        buckets.set(time, { time });
      }

      const row = buckets.get(time)!;

      row[`${key}:utilization`] = point.utilization;
      row[`${key}:memory`] = point.memory_used_mb;
      row[`${key}:temperature`] = point.temperature;
    }

    return [...buckets.values()].sort(
      (a, b) => a.time - b.time,
    );
  }, [points]);

  return (
    <div className="panel">
      <div className="node-toolbar">
        <div>
          <h2>全部 GPU 运行趋势</h2>
          <p>
            所有 GPU 历史曲线对比 · 每 10 秒刷新 ·
            当前 {devices.length} 张 GPU
          </p>
        </div>

        <div className="cluster-range-switch">
          {(["1h", "6h", "24h"] as const).map((value) => (
            <button
              key={value}
              type="button"
              className={range === value ? "active" : ""}
              onClick={() => setRange(value)}
            >
              {value === "1h"
                ? "1 小时"
                : value === "6h"
                  ? "6 小时"
                  : "24 小时"}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <p style={{ color: "#f87171" }}>
          GPU 历史数据获取失败：{error}
        </p>
      )}

      {loading && chartData.length === 0 ? (
        <p className="empty-state">正在加载 GPU 历史数据...</p>
      ) : devices.length === 0 ? (
        <p className="empty-state">当前没有可用的 GPU 设备</p>
      ) : chartData.length === 0 ? (
        <p className="empty-state">当前时间范围暂无历史数据</p>
      ) : (
        <div className="cluster-history-grid">
          {metrics.map((metric) => (
            <div
              className="cluster-history-card"
              key={metric.key}
            >
              <h3>{metric.title}</h3>

              <div style={{ width: "100%", height: 290 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={chartData}>
                    <CartesianGrid
                      stroke="#263650"
                      strokeDasharray="3 3"
                    />

                    <XAxis
                      dataKey="time"
                      type="number"
                      domain={["dataMin", "dataMax"]}
                      tickFormatter={formatTime}
                      stroke="#94a3b8"
                      fontSize={11}
                      minTickGap={25}
                    />

                    <YAxis
                      stroke="#94a3b8"
                      fontSize={11}
                      width={48}
                      domain={
                        metric.key === "utilization"
                          ? [0, 100]
                          : [0, "auto"]
                      }
                    />

                    <Tooltip
                      labelFormatter={(value) =>
                        new Date(Number(value)).toLocaleString("zh-CN")
                      }
                      contentStyle={{
                        background: "#172238",
                        border: "1px solid #334155",
                        borderRadius: 8,
                        color: "#fff",
                      }}
                    />

                    <Legend
                      wrapperStyle={{ fontSize: 11 }}
                    />

                    {devices.map((gpu, index) => (
                      <Line
                        key={`${gpu.key}:${metric.key}`}
                        type="monotone"
                        dataKey={`${gpu.key}:${metric.key}`}
                        name={`${gpu.hostname} / GPU ${gpu.gpuIndex}`}
                        stroke={colors[index % colors.length]}
                        strokeWidth={2}
                        dot={false}
                        isAnimationActive={false}
                        connectNulls={false}
                        unit={metric.unit}
                      />
                    ))}
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

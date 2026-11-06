import { useEffect, useState } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

type GPUHistoryRecord = {
  id: number;
  node_id: string;
  gpu_index: number;
  name: string;
  utilization: number;
  memory_total_mb: number;
  memory_used_mb: number;
  temperature: number;
  collected_at: string;
};

type GPUHistoryResponse = {
  total: number;
  limit: number;
  offset: number;
  records: GPUHistoryRecord[];
};

type Props = {
  nodeId: string;
  gpuIndex?: number;
};

type TimeRange = 1 | 6 | 24;

const ranges: TimeRange[] = [1, 6, 24];

export default function GPUHistoryChart({ nodeId, gpuIndex = 0 }: Props) {
  const [range, setRange] = useState<TimeRange>(1);
  const [records, setRecords] = useState<GPUHistoryRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();

    async function loadHistory() {
      try {
        const to = new Date();
        const from = new Date(to.getTime() - range * 60 * 60 * 1000);

        const allRecords: GPUHistoryRecord[] = [];
        const pageSize = 100;
        const maxRecords = 20000;
        let offset = 0;

        while (offset < maxRecords) {
          const params = new URLSearchParams({
            gpu_index: String(gpuIndex),
            from: from.toISOString(),
            to: to.toISOString(),
            limit: String(pageSize),
            offset: String(offset),
          });

          const response = await fetch(
            `/api/v1/nodes/${encodeURIComponent(nodeId)}/gpus/history?${params}`,
            { signal: controller.signal },
          );

          if (!response.ok) {
            throw new Error(`历史数据请求失败：HTTP ${response.status}`);
          }

          const result: GPUHistoryResponse = await response.json();
          const pageRecords = result.records ?? [];

          allRecords.push(...pageRecords);
          offset += pageRecords.length;

          if (pageRecords.length === 0 || offset >= result.total) {
            break;
          }
        }

        // 按时间从旧到新排序
        allRecords.sort(
          (a, b) =>
            new Date(a.collected_at).getTime() -
            new Date(b.collected_at).getTime(),
        );

        // 降采样：最多保留约 500 个绘图点
        const maxPoints = 500;
        const step = Math.max(1, Math.ceil(allRecords.length / maxPoints));

        const sampled = allRecords.filter((_, index) => index % step === 0);

        // 保留最新采样点
        const latest = allRecords[allRecords.length - 1];
        if (latest && sampled[sampled.length - 1]?.id !== latest.id) {
          sampled.push(latest);
        }

        if (!controller.signal.aborted) {
          setRecords(sampled);
          setError("");
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(err instanceof Error ? err.message : "加载失败");
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    }

    void loadHistory();

    const timer = window.setInterval(() => {
      void loadHistory();
    }, 10000);

    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [nodeId, gpuIndex, range]);

  const data = records.map((record) => ({
    timestamp: new Date(record.collected_at).getTime(),
    utilization: record.utilization,
    memory: record.memory_used_mb,
    temperature: record.temperature,
  }));

  const formatTime = (value: number) =>
    new Date(value).toLocaleTimeString("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
    });

  const charts = [
    {
      title: "GPU 利用率",
      key: "utilization",
      color: "#38bdf8",
      unit: "%",
      domain: [0, 100],
    },
    {
      title: "显存使用量",
      key: "memory",
      color: "#34d399",
      unit: "MB",
      domain: [0, "auto"],
    },
    {
      title: "GPU 温度",
      key: "temperature",
      color: "#a78bfa",
      unit: "°C",
      domain: [0, "auto"],
    },
  ] as const;

  return (
    <div className="history-container">
      <div className="history-header">
        <div>
          <h2>GPU 运行趋势</h2>
          <p>
            历史采样数据 · 每 10 秒刷新 · 当前展示 {records.length} 个采样点
          </p>
        </div>

        <div className="history-range">
          {ranges.map((hours) => (
            <button
              key={hours}
              type="button"
              className={range === hours ? "selected" : ""}
              onClick={() => {
                setLoading(true);
                setRange(hours);
              }}
            >
              {hours} 小时
            </button>
          ))}
        </div>
      </div>

      {error && <p className="history-error">{error}</p>}

      {loading && <p className="history-message">正在加载历史数据...</p>}

      {!loading && data.length === 0 && !error && (
        <p className="history-message">当前时间范围内暂无 GPU 历史数据</p>
      )}

      {data.length > 0 && (
        <div className="history-charts">
          {charts.map((chart) => (
            <div className="history-chart" key={chart.key}>
              <h3>{chart.title}</h3>

              <ResponsiveContainer width="100%" height={230}>
                <LineChart
                  data={data}
                  margin={{ top: 10, right: 18, left: 0, bottom: 0 }}
                >
                  <CartesianGrid stroke="#28364d" strokeDasharray="3 3" />

                  <XAxis
                    dataKey="timestamp"
                    type="number"
                    scale="time"
                    domain={["dataMin", "dataMax"]}
                    tickFormatter={formatTime}
                    stroke="#94a3b8"
                    fontSize={11}
                  />

                  <YAxis
                    domain={[...chart.domain]}
                    stroke="#94a3b8"
                    fontSize={11}
                    width={45}
                  />

                  <Tooltip
                    labelFormatter={(label) =>
                      new Date(Number(label)).toLocaleString("zh-CN")
                    }
                    formatter={(value) => [
                      `${value ?? "—"} ${chart.unit}`,
                      chart.title,
                    ]}
                    contentStyle={{
                      background: "#172238",
                      border: "1px solid #334155",
                      borderRadius: 8,
                      color: "#fff",
                    }}
                  />

                  <Line
                    type="monotone"
                    dataKey={chart.key}
                    stroke={chart.color}
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4 }}
                    isAnimationActive={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

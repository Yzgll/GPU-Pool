
import { useEffect, useMemo, useState } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

type TimeRange = "1h" | "6h" | "24h";

type HistoryPoint = {
  collected_at: string;
  gpu_count: number;
  avg_utilization: number;
  memory_used_mb: number;
  memory_total_mb: number;
  avg_temperature: number;
  total_power_w: number;
};

type HistoryResponse = {
  range: string;
  interval_seconds: number;
  points: HistoryPoint[];
};

const charts = [
  {
    key: "utilization",
    title: "平均 GPU 利用率",
    color: "#38bdf8",
    unit: "%",
  },
  {
    key: "memory",
    title: "集群显存使用量",
    color: "#34d399",
    unit: " MB",
  },
  {
    key: "temperature",
    title: "平均 GPU 温度",
    color: "#a78bfa",
    unit: " °C",
  },
] as const;

export default function GPUClusterHistoryChart() {
  const [range, setRange] = useState<TimeRange>("1h");
  const [points, setPoints] = useState<HistoryPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();

    async function loadHistory() {
      try {
        const response = await fetch(
          `/api/v1/gpus/history/summary?range=${range}`,
          { signal: controller.signal },
        );

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const data: HistoryResponse = await response.json();

        if (!controller.signal.aborted) {
          setPoints(data.points ?? []);
          setError("");
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(
            err instanceof Error
              ? err.message
              : "获取集群历史数据失败",
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
  }, [range]);

  const data = useMemo(
    () =>
      points.map((point) => ({
        time: new Date(point.collected_at).getTime(),
        utilization: point.avg_utilization,
        memory: point.memory_used_mb,
        temperature: point.avg_temperature,
      })),
    [points],
  );

  return (
    <div className="panel">
      <div className="node-toolbar">
        <div>
          <h2>集群 GPU 运行总览</h2>
          <p>所有 GPU 聚合历史数据 · 每 10 秒刷新</p>
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
          集群历史数据获取失败：{error}
        </p>
      )}

      {loading && data.length === 0 ? (
        <p className="empty-state">
          正在加载集群历史数据...
        </p>
      ) : data.length === 0 ? (
        <p className="empty-state">
          当前时间范围暂无集群历史数据
        </p>
      ) : (
        <div className="cluster-history-grid">
          {charts.map((chart) => (
            <div
              className="cluster-history-card"
              key={chart.key}
            >
              <h3>{chart.title}</h3>

              <div style={{ width: "100%", height: 290 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={data}>
                    <CartesianGrid
                      stroke="#263650"
                      strokeDasharray="3 3"
                    />

                    <XAxis
                      dataKey="time"
                      type="number"
                      domain={["dataMin", "dataMax"]}
                      tickFormatter={(value: number) =>
                        new Date(value).toLocaleTimeString(
                          "zh-CN",
                          {
                            hour: "2-digit",
                            minute: "2-digit",
                          },
                        )
                      }
                      stroke="#94a3b8"
                      fontSize={11}
                      minTickGap={25}
                    />

                    <YAxis
                      stroke="#94a3b8"
                      fontSize={11}
                      width={48}
                      domain={
                        chart.key === "utilization"
                          ? [0, 100]
                          : [0, "auto"]
                      }
                    />

                    <Tooltip
                      labelFormatter={(value) =>
                        new Date(Number(value)).toLocaleString(
                          "zh-CN",
                        )
                      }
                      formatter={(value) => [
                        `${Number(value).toFixed(1)}${chart.unit}`,
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
                      name={chart.title}
                      stroke={chart.color}
                      strokeWidth={2}
                      dot={false}
                      isAnimationActive={false}
                      connectNulls={false}
                    />
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

import { useId, useMemo } from "react";

export type MetricDay = { date: string; value: number };

const W = 560;
const PAD = { top: 8, right: 8, bottom: 28, left: 44 };

type MetricKind = "rss" | "cpu";

type Props = {
  days: MetricDay[];
  kind: MetricKind;
  legend: string;
  compact?: boolean;
};

export function StatsMetricDayChart({
  days,
  kind,
  legend,
  compact = true,
}: Props) {
  const H = compact ? 108 : 160;
  const grad = useId();

  const colors =
    kind === "rss"
      ? { a: "#c07ae8", b: "#d8a0f0", legendClass: "stats-legend-rss" }
      : { a: "#e0a060", b: "#eeb080", legendClass: "stats-legend-cpu" };

  const layout = useMemo(() => {
    if (!days.length) {
      return null;
    }
    const maxVal = Math.max(
      kind === "cpu" ? 0.1 : 1,
      ...days.map((d) => d.value),
    );
    const plotW = W - PAD.left - PAD.right;
    const plotH = H - PAD.top - PAD.bottom;
    const groupW = plotW / days.length;
    const barW = Math.min(compact ? 14 : 16, Math.max(4, groupW * 0.55));

    const bars = days.map((d, i) => {
      const cx = PAD.left + groupW * i + groupW / 2;
      const barH = (d.value / maxVal) * plotH;
      const baseY = PAD.top + plotH;
      return {
        date: d.date,
        label: d.date.slice(5),
        bar: {
          x: cx - barW / 2,
          y: baseY - barH,
          h: barH,
        },
        showLabel: days.length <= 14 || i % Math.ceil(days.length / 10) === 0,
      };
    });

    const gridSteps = compact ? [0, 0.5, 1] : [0, 0.25, 0.5, 0.75, 1];
    const gridLines = gridSteps.map((t) => ({
      y: PAD.top + plotH * (1 - t),
      label: fmtAxis(maxVal * t, kind),
    }));

    return { bars, barW, plotH, gridLines, H };
  }, [days, compact, kind]);

  if (!layout) {
    return (
      <div className="stats-chart-empty">Нет данных за выбранный период</div>
    );
  }

  const { bars, barW, gridLines, H: height } = layout;
  const aria =
    kind === "rss"
      ? "График RAM по дням"
      : "График CPU по дням";

  return (
    <div className="stats-chart">
      <svg
        className="stats-chart-svg"
        viewBox={`0 0 ${W} ${height}`}
        role="img"
        aria-label={aria}
      >
        <defs>
          <linearGradient id={grad} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={colors.a} />
            <stop offset="100%" stopColor={colors.b} />
          </linearGradient>
        </defs>
        {gridLines.map((g) => (
          <g key={g.y}>
            <line
              x1={PAD.left}
              y1={g.y}
              x2={W - PAD.right}
              y2={g.y}
              className="stats-chart-grid"
            />
            <text
              x={PAD.left - 6}
              y={g.y + 3}
              textAnchor="end"
              className="stats-chart-axis"
            >
              {g.label}
            </text>
          </g>
        ))}
        {bars.map((b) => (
          <g key={b.date}>
            <rect
              x={b.bar.x}
              y={b.bar.y}
              width={barW}
              height={Math.max(0, b.bar.h)}
              rx={2}
              fill={`url(#${grad})`}
            />
            {b.showLabel ? (
              <text
                x={b.bar.x + barW / 2}
                y={height - 8}
                textAnchor="middle"
                className="stats-chart-label"
              >
                {b.label}
              </text>
            ) : null}
          </g>
        ))}
      </svg>
      <div className="stats-chart-legend">
        <span>
          <i className={colors.legendClass} /> {legend}
        </span>
      </div>
    </div>
  );
}

function fmtAxis(v: number, kind: MetricKind): string {
  if (kind === "cpu") {
    if (v < 0.05) {
      return "0%";
    }
    return `${v.toFixed(v >= 10 ? 0 : 1)}%`;
  }
  if (v < 1) {
    return "0";
  }
  if (v >= 1024 * 1024) {
    return `${(v / (1024 * 1024)).toFixed(0)}G`;
  }
  if (v >= 1024) {
    return `${(v / 1024).toFixed(0)}M`;
  }
  return `${Math.round(v)}K`;
}
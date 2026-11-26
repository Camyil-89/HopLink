import { useId, useMemo } from "react";

export type ChartDay = { date: string; in: number; out: number };

const W = 560;
const PAD = { top: 8, right: 8, bottom: 28, left: 38 };

type Props = {
  days: ChartDay[];
  title?: string;
  compact?: boolean;
};

export function StatsTrafficChart({ days, title, compact = true }: Props) {
  const H = compact ? 108 : 160;
  const gradIn = useId();
  const gradOut = useId();

  const layout = useMemo(() => {
    if (!days.length) {
      return null;
    }
    const maxVal = Math.max(1, ...days.flatMap((d) => [d.in, d.out]));
    const plotW = W - PAD.left - PAD.right;
    const plotH = H - PAD.top - PAD.bottom;
    const groupW = plotW / days.length;
    const barW = Math.min(compact ? 10 : 12, Math.max(3, groupW * 0.26));
    const gap = 2;

    const bars = days.map((d, i) => {
      const cx = PAD.left + groupW * i + groupW / 2;
      const inH = (d.in / maxVal) * plotH;
      const outH = (d.out / maxVal) * plotH;
      const baseY = PAD.top + plotH;
      return {
        date: d.date,
        label: d.date.slice(5),
        inBar: {
          x: cx - barW - gap / 2,
          y: baseY - inH,
          h: inH,
        },
        outBar: {
          x: cx + gap / 2,
          y: baseY - outH,
          h: outH,
        },
        showLabel: days.length <= 14 || i % Math.ceil(days.length / 10) === 0,
      };
    });

    const gridSteps = compact ? [0, 0.5, 1] : [0, 0.25, 0.5, 0.75, 1];
    const gridLines = gridSteps.map((t) => ({
      y: PAD.top + plotH * (1 - t),
      label: fmtAxis(maxVal * t),
    }));

    return { bars, barW, maxVal, plotH, PAD, gridLines, groupW, H };
  }, [days, compact]);

  if (!layout) {
    return (
      <div className="stats-chart-empty">Нет данных за выбранный период</div>
    );
  }

  const { bars, barW, PAD: p, gridLines, H: height } = layout;

  return (
    <div className="stats-chart">
      {title ? <div className="stats-chart-title">{title}</div> : null}
      <svg
        className="stats-chart-svg"
        viewBox={`0 0 ${W} ${height}`}
        role="img"
        aria-label="График трафика по дням"
      >
        <defs>
          <linearGradient id={gradIn} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#6b7df0" />
            <stop offset="100%" stopColor="#8b97f4" />
          </linearGradient>
          <linearGradient id={gradOut} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#52bd8c" />
            <stop offset="100%" stopColor="#6dd4a8" />
          </linearGradient>
        </defs>
        {gridLines.map((g) => (
          <g key={g.y}>
            <line
              x1={p.left}
              y1={g.y}
              x2={W - p.right}
              y2={g.y}
              className="stats-chart-grid"
            />
            <text
              x={p.left - 6}
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
              x={b.inBar.x}
              y={b.inBar.y}
              width={barW}
              height={Math.max(0, b.inBar.h)}
              rx={2}
              fill={`url(#${gradIn})`}
            />
            <rect
              x={b.outBar.x}
              y={b.outBar.y}
              width={barW}
              height={Math.max(0, b.outBar.h)}
              rx={2}
              fill={`url(#${gradOut})`}
            />
            {b.showLabel ? (
              <text
                x={b.inBar.x + barW + 2}
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
          <i className="stats-legend-in" /> Входящий
        </span>
        <span>
          <i className="stats-legend-out" /> Исходящий
        </span>
      </div>
    </div>
  );
}

function fmtAxis(kb: number): string {
  if (kb >= 1024 * 1024) {
    return `${(kb / (1024 * 1024)).toFixed(0)}G`;
  }
  if (kb >= 1024) {
    return `${(kb / 1024).toFixed(0)}M`;
  }
  return `${Math.round(kb)}K`;
}
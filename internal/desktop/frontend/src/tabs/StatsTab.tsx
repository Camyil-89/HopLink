import { RefreshCw, BarChart3 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { api } from "../app/api";
import { StatTile } from "../components/StatTile";
import { StatsMetricDayChart } from "../components/StatsMetricDayChart";
import { StatsTrafficChart } from "../components/StatsTrafficChart";
import type { AppState } from "../types/app";
import {
  fmtKB,
  fmtPct,
  normalizeHop,
  type HopStat,
} from "./statsHopData";
import {
  PERIOD_OPTIONS,
  daySeriesForPeriod,
  mergeDaysByDate,
  processDaySeriesForPeriod,
  sumDays,
  type PeriodOption,
} from "./statsPeriod";

type Props = {
  state: AppState;
  busy: boolean;
};

export function StatsTab({ state, busy }: Props) {
  const [hops, setHops] = useState<HopStat[]>([]);
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);
  const [period, setPeriod] = useState<PeriodOption>(14);

  const load = useCallback(async () => {
    if (!state.activeRouteId) {
      return;
    }
    setLoading(true);
    setMessage("");
    try {
      const raw = await api().GetRouteTrafficStats(state.activeRouteId);
      setHops((raw ?? []).map((row) => normalizeHop(row)));
    } catch (err) {
      setMessage(err instanceof Error ? err.message : String(err));
      setHops([]);
    } finally {
      setLoading(false);
    }
  }, [state.activeRouteId]);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(timer);
  }, [load]);

  const periodLabel =
    PERIOD_OPTIONS.find((o) => o.value === period)?.label ?? "14 дней";

  const hopOk = (h: HopStat) => !h.missing && !h.error;
  const routeDayRows = mergeDaysByDate(
    hops.filter(hopOk).flatMap((h) => h.stats.days),
  );
  const routeDays = daySeriesForPeriod(routeDayRows, period);
  const routeTotals = sumDays(routeDays);

  return (
    <section className="content-page route-stats">
      <div className="page-head">
        <div>
          <div className="eyebrow">
            СТАТИСТИКА <span>•</span> {hops.length} узлов
          </div>
          <h1>Узлы и ресурсы</h1>
          <p>
            Трафик, RAM и CPU процесса hoplink по серверам. Обновление каждые 30
            секунд.
          </p>
        </div>
        <div className="page-head-actions">
          <label className="stats-period-select">
            <span>Период</span>
            <select
              value={period}
              onChange={(e) =>
                setPeriod(Number(e.target.value) as PeriodOption)
              }
            >
              {PERIOD_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
          <button
            className="btn ghost"
            disabled={busy || loading}
            onClick={() => void load()}
          >
            <RefreshCw size={16} /> Обновить
          </button>
        </div>
      </div>
      {message ? <div className="notice-banner">{message}</div> : null}
      {!hops.length && !message ? (
        <div className="table-card">
          <div className="empty-table">
            <BarChart3 size={28} />
            <b>Нет данных</b>
            <small>
              Установите маршрут на серверы или дождитесь накопления трафика.
            </small>
          </div>
        </div>
      ) : null}
      {routeDays.length > 0 && (routeTotals.in > 0 || routeTotals.out > 0) ? (
        <div className="table-card stats-route-card">
          <div className="table-caption">
            <div>
              <b>Маршрут целиком</b>
              <small>Сумма по узлам · {periodLabel}</small>
            </div>
          </div>
          <div className="stats-hop-body">
            <div className="stats-summary stats-summary-compact">
              <StatTile
                label={`Входящий, ${periodLabel.toLowerCase()}`}
                value={fmtKB(routeTotals.in)}
              />
              <StatTile
                label={`Исходящий, ${periodLabel.toLowerCase()}`}
                value={fmtKB(routeTotals.out)}
              />
            </div>
            <div className="stats-chart-wrap stats-chart-wrap-compact">
              <StatsTrafficChart days={routeDays} compact />
            </div>
          </div>
        </div>
      ) : null}
      <div className="stats-hops-grid">
        {hops.map((hop) => (
          <HopStatsCard
            key={hop.serverId}
            hop={hop}
            period={period}
            periodLabel={periodLabel}
          />
        ))}
      </div>
    </section>
  );
}

function HopStatsCard({
  hop,
  period,
  periodLabel,
}: {
  hop: HopStat;
  period: PeriodOption;
  periodLabel: string;
}) {
  const showStats = !hop.missing && !hop.error;
  const chartDays = daySeriesForPeriod(hop.stats.days, period);
  const periodTotals = sumDays(chartDays);

  const process = hop.stats.process;
  const processDays = process?.days ?? [];
  const processChartDays = processDaySeriesForPeriod(processDays, period);
  const showProcess =
    process &&
    (process.current.rssKB > 0 ||
      process.current.cpuPct > 0 ||
      processDays.length > 0);

  return (
    <div className="table-card stats-hop-card">
      <div className="table-caption">
        <div>
          <b>
            {hop.serverName || hop.serverHost}{" "}
            <small>({hop.serverHost})</small>
          </b>
          {hop.error || hop.missing ? (
            <small>
              {hop.error ||
                (hop.statsPath
                  ? `Файл не найден: ${hop.statsPath}`
                  : "Файл stats.json не найден")}
            </small>
          ) : null}
        </div>
      </div>
      {showStats ? (
        <div className="stats-hop-body">
          <div className="stats-summary stats-summary-compact">
            <StatTile
              label={`Входящий, ${periodLabel.toLowerCase()}`}
              value={fmtKB(periodTotals.in)}
            />
            <StatTile
              label={`Исходящий, ${periodLabel.toLowerCase()}`}
              value={fmtKB(periodTotals.out)}
            />
          </div>
          {chartDays.length > 0 ? (
            <div className="stats-chart-wrap stats-chart-wrap-compact">
              <StatsTrafficChart days={chartDays} compact />
            </div>
          ) : null}
          {showProcess ? (
            <div className="stats-process-block">
              <div className="stats-process-head">Процесс hoplink</div>
              <div className="stats-summary stats-summary-compact">
                <StatTile
                  label="RAM сейчас"
                  value={fmtKB(process.current.rssKB)}
                />
                <StatTile
                  label="CPU сейчас"
                  value={fmtPct(process.current.cpuPct)}
                />
              </div>
              {processChartDays.length > 0 ? (
                <>
                  <div className="stats-chart-wrap stats-chart-wrap-compact stats-chart-wrap-process">
                    <StatsMetricDayChart
                      kind="rss"
                      legend="RAM, пик за день"
                      days={processChartDays.map((d) => ({
                        date: d.date,
                        value: d.rssMaxKB,
                      }))}
                      compact
                    />
                  </div>
                  <div className="stats-chart-wrap stats-chart-wrap-compact stats-chart-wrap-process">
                    <StatsMetricDayChart
                      kind="cpu"
                      legend="CPU, среднее за день"
                      days={processChartDays.map((d) => ({
                        date: d.date,
                        value: d.cpuAvg,
                      }))}
                      compact
                    />
                  </div>
                </>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
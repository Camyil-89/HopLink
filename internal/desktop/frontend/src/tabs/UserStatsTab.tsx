import { Activity, RefreshCw, Users } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../app/api";
import { StatTile } from "../components/StatTile";
import { StatsTrafficChart } from "../components/StatsTrafficChart";
import type { AppState, User } from "../types/app";
import {
  fmtKB,
  normalizeHop,
  type HopStat,
  type UserTrafficRow,
} from "./statsHopData";
import {
  PERIOD_OPTIONS,
  daySeriesForPeriod,
  mergeDaysByDate,
  mergeRouteUsers,
  sumDays,
  type PeriodOption,
} from "./statsPeriod";

type DisplayUser = {
  key: string;
  label: string;
  active: boolean;
  stats: UserTrafficRow | null;
};

function buildDisplayUsers(
  configured: User[],
  fromStats: UserTrafficRow[],
): DisplayUser[] {
  const byKey = new Map(fromStats.map((u) => [u.id, u]));
  const seen = new Set<string>();
  const rows: DisplayUser[] = [];

  for (const u of configured) {
    const stats =
      byKey.get(u.name) ?? byKey.get(u.id) ?? null;
    const key = stats?.id ?? u.name;
    if (stats) {
      seen.add(stats.id);
    }
    rows.push({
      key,
      label: u.name,
      active: u.active !== false,
      stats,
    });
  }

  for (const s of fromStats) {
    if (seen.has(s.id)) {
      continue;
    }
    rows.push({
      key: s.id,
      label: s.id,
      active: true,
      stats: s,
    });
  }

  return rows;
}

type Props = {
  state: AppState;
  busy: boolean;
};

export function UserStatsTab({ state, busy }: Props) {
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
  const mergedUsers = useMemo(
    () => mergeRouteUsers(hops.filter(hopOk)),
    [hops],
  );

  const displayUsers = useMemo(
    () => buildDisplayUsers(state.users, mergedUsers),
    [state.users, mergedUsers],
  );

  const routeUserDays = daySeriesForPeriod(
    mergeDaysByDate(mergedUsers.flatMap((u) => u.days)),
    period,
  );
  const routeTotals = sumDays(routeUserDays);

  return (
    <section className="content-page route-stats route-user-stats">
      <div className="page-head">
        <div>
          <div className="eyebrow">
            КЛИЕНТЫ <span>•</span> {displayUsers.length}
          </div>
          <h1>Трафик пользователей</h1>
          <p>
            Статистика по учётным записям маршрута с узлов, где включён сбор
            stats.json. Обновление каждые 30 секунд.
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
      {!displayUsers.length && !message ? (
        <div className="table-card">
          <div className="empty-table">
            <Users size={28} />
            <b>Нет пользователей</b>
            <small>
              Добавьте клиентов на вкладке «Пользователи» или дождитесь
              накопления трафика.
            </small>
          </div>
        </div>
      ) : null}
      {routeUserDays.length > 0 &&
      (routeTotals.in > 0 || routeTotals.out > 0) ? (
        <div className="table-card stats-route-card">
          <div className="table-caption">
            <div>
              <b>Все клиенты</b>
              <small>{periodLabel}</small>
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
              <StatsTrafficChart days={routeUserDays} compact />
            </div>
          </div>
        </div>
      ) : null}
      <div className="stats-hops-grid stats-users-grid">
        {displayUsers.map((user) => (
          <UserTrafficCard
            key={user.key}
            user={user}
            period={period}
            periodLabel={periodLabel}
          />
        ))}
      </div>
    </section>
  );
}

function UserTrafficCard({
  user,
  period,
  periodLabel,
}: {
  user: DisplayUser;
  period: PeriodOption;
  periodLabel: string;
}) {
  const chartDays = daySeriesForPeriod(user.stats?.days ?? [], period);
  const periodTotals = sumDays(chartDays);
  const hasTraffic =
    periodTotals.in > 0 ||
    periodTotals.out > 0 ||
    (user.stats && (user.stats.in > 0 || user.stats.out > 0));

  return (
    <div className="table-card stats-hop-card stats-user-card">
      <div className="table-caption">
        <div>
          <b>
            {user.label}{" "}
            {!user.active ? (
              <small className="stats-user-inactive">неактивен</small>
            ) : null}
          </b>
          <small>
            {user.stats
              ? `Всего: ${fmtKB(user.stats.in)} / ${fmtKB(user.stats.out)}`
              : "Нет данных на узлах"}
          </small>
        </div>
      </div>
      <div className="stats-hop-body">
        {hasTraffic ? (
          <>
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
          </>
        ) : (
          <div className="stats-chart-empty stats-user-empty">
            <Activity size={22} />
            <span>Трафик за период не зафиксирован</span>
          </div>
        )}
      </div>
    </div>
  );
}
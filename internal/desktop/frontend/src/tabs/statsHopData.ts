import { desktop } from "../../wailsjs/go/models";
import type { DayRow, ProcessDayRow } from "./statsPeriod";

export type UserTrafficRow = {
  id: string;
  in: number;
  out: number;
  days: DayRow[];
};

export type ProcessStat = {
  current: { rssKB: number; cpuPct: number };
  days: ProcessDayRow[];
};

export type HopStat = {
  serverId: string;
  serverHost: string;
  serverName: string;
  serviceDir: string;
  statsPath: string;
  missing: boolean;
  error: string;
  stats: {
    total: { in: number; out: number };
    days: DayRow[];
    users: UserTrafficRow[];
    process?: ProcessStat;
  };
};

function num(v: unknown): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

export function normalizeHop(row: unknown): HopStat {
  const h = desktop.RouteHopStats.createFrom(row);
  const raw = (row ?? {}) as Record<string, unknown>;
  const srcStats = (raw.stats ?? {}) as Record<string, unknown>;
  const totalSrc = (srcStats.total ?? {}) as Record<string, unknown>;
  const daysSrc = Array.isArray(srcStats.days) ? srcStats.days : [];
  const usersSrc = Array.isArray(srcStats.users) ? srcStats.users : [];
  const procSrc = (srcStats.process ?? null) as Record<string, unknown> | null;
  const procCurrent = (procSrc?.current ?? {}) as Record<string, unknown>;
  const procDaysSrc = Array.isArray(procSrc?.days) ? procSrc.days : [];

  return {
    serverId: h.serverId ?? "",
    serverHost: h.serverHost ?? "",
    serverName: h.serverName ?? "",
    serviceDir: h.serviceDir ?? "",
    statsPath: h.statsPath ?? "",
    missing: Boolean(h.missing),
    error: typeof h.error === "string" ? h.error : "",
    stats: {
      total: { in: num(totalSrc.in), out: num(totalSrc.out) },
      days: daysSrc.map((d) => {
        const day = d as Record<string, unknown>;
        return {
          date: String(day.date ?? ""),
          in: num(day.in),
          out: num(day.out),
        };
      }),
      users: usersSrc.map((u) => {
        const user = u as Record<string, unknown>;
        const uDays = Array.isArray(user.days) ? user.days : [];
        return {
          id: String(user.id ?? ""),
          in: num(user.in),
          out: num(user.out),
          days: uDays.map((d) => {
            const day = d as Record<string, unknown>;
            return {
              date: String(day.date ?? ""),
              in: num(day.in),
              out: num(day.out),
            };
          }),
        };
      }),
      process: procSrc
        ? {
            current: {
              rssKB: num(procCurrent.rssKB),
              cpuPct: num(procCurrent.cpuPct),
            },
            days: procDaysSrc.map((d) => {
              const day = d as Record<string, unknown>;
              const rss = (day.rssKB ?? {}) as Record<string, unknown>;
              const cpu = (day.cpuPct ?? {}) as Record<string, unknown>;
              return {
                date: String(day.date ?? ""),
                rssLastKB: num(rss.last),
                rssMaxKB: num(rss.max),
                cpuAvg: num(cpu.avg),
                cpuMax: num(cpu.max),
              };
            }),
          }
        : undefined,
    },
  };
}

export function fmtKB(v: number) {
  if (v >= 1024 * 1024) {
    return `${(v / (1024 * 1024)).toFixed(2)} ГБ`;
  }
  if (v >= 1024) {
    return `${(v / 1024).toFixed(1)} МБ`;
  }
  return `${v} КБ`;
}

export function fmtPct(v: number) {
  return `${v.toFixed(1)}%`;
}
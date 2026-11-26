export type PeriodOption = 7 | 14 | 30 | 90 | 0;

export const PERIOD_OPTIONS: { value: PeriodOption; label: string }[] = [
  { value: 7, label: "7 дней" },
  { value: 14, label: "14 дней" },
  { value: 30, label: "30 дней" },
  { value: 90, label: "90 дней" },
  { value: 0, label: "Всё время" },
];

export type DayRow = { date: string; in: number; out: number };

export type ProcessDayRow = {
  date: string;
  rssMaxKB: number;
  rssLastKB: number;
  cpuAvg: number;
  cpuMax: number;
};

function parseDay(date: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) {
    return null;
  }
  const d = new Date(`${date}T12:00:00`);
  return Number.isNaN(d.getTime()) ? null : d;
}

function todayLocal(): Date {
  const d = new Date();
  d.setHours(12, 0, 0, 0);
  return d;
}

function formatDay(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/** Builds a continuous day series for charts (zeros on missing days). */
export function daySeriesForPeriod(
  days: DayRow[],
  period: PeriodOption,
): DayRow[] {
  const byDate = new Map(days.map((d) => [d.date, d]));
  if (period === 0) {
    if (!days.length) {
      return [];
    }
    const sorted = [...days].sort((a, b) => a.date.localeCompare(b.date));
    const first = parseDay(sorted[0].date);
    const last = parseDay(sorted[sorted.length - 1].date);
    if (!first || !last) {
      return sorted;
    }
    const out: DayRow[] = [];
    for (let cur = new Date(first); cur <= last; cur.setDate(cur.getDate() + 1)) {
      const key = formatDay(cur);
      const row = byDate.get(key);
      out.push(row ?? { date: key, in: 0, out: 0 });
    }
    return out;
  }
  const out: DayRow[] = [];
  const end = todayLocal();
  for (let i = period - 1; i >= 0; i--) {
    const d = new Date(end);
    d.setDate(d.getDate() - i);
    const key = formatDay(d);
    const row = byDate.get(key);
    out.push(row ?? { date: key, in: 0, out: 0 });
  }
  return out;
}

export function sumDays(days: DayRow[]): { in: number; out: number } {
  return days.reduce(
    (acc, d) => ({ in: acc.in + d.in, out: acc.out + d.out }),
    { in: 0, out: 0 },
  );
}

/** Builds a continuous process day series for charts. */
export function processDaySeriesForPeriod(
  days: ProcessDayRow[],
  period: PeriodOption,
): ProcessDayRow[] {
  const byDate = new Map(days.map((d) => [d.date, d]));
  const empty = (key: string): ProcessDayRow => ({
    date: key,
    rssMaxKB: 0,
    rssLastKB: 0,
    cpuAvg: 0,
    cpuMax: 0,
  });
  if (period === 0) {
    if (!days.length) {
      return [];
    }
    const sorted = [...days].sort((a, b) => a.date.localeCompare(b.date));
    const first = parseDay(sorted[0].date);
    const last = parseDay(sorted[sorted.length - 1].date);
    if (!first || !last) {
      return sorted;
    }
    const out: ProcessDayRow[] = [];
    for (let cur = new Date(first); cur <= last; cur.setDate(cur.getDate() + 1)) {
      const key = formatDay(cur);
      out.push(byDate.get(key) ?? empty(key));
    }
    return out;
  }
  const out: ProcessDayRow[] = [];
  const end = todayLocal();
  for (let i = period - 1; i >= 0; i--) {
    const d = new Date(end);
    d.setDate(d.getDate() - i);
    const key = formatDay(d);
    out.push(byDate.get(key) ?? empty(key));
  }
  return out;
}

export type UserTrafficMerge = {
  id: string;
  in: number;
  out: number;
  days: DayRow[];
};

/** Merges per-user traffic from several hops (sums daily and total counters). */
export function mergeRouteUsers(
  hops: { stats: { users: UserTrafficMerge[] } }[],
): UserTrafficMerge[] {
  const byId = new Map<string, UserTrafficMerge>();
  for (const hop of hops) {
    for (const u of hop.stats.users) {
      if (!u.id || u.id === "relay") {
        continue;
      }
      const existing = byId.get(u.id);
      if (!existing) {
        byId.set(u.id, {
          id: u.id,
          in: u.in,
          out: u.out,
          days: u.days.map((d) => ({ ...d })),
        });
        continue;
      }
      existing.in += u.in;
      existing.out += u.out;
      existing.days = mergeDaysByDate([...existing.days, ...u.days]);
    }
  }
  return [...byId.values()].sort(
    (a, b) => b.in + b.out - (a.in + a.out),
  );
}

export function mergeDaysByDate(days: DayRow[]): DayRow[] {
  const byDate = new Map<string, DayRow>();
  for (const d of days) {
    const cur = byDate.get(d.date) ?? { date: d.date, in: 0, out: 0 };
    cur.in += d.in;
    cur.out += d.out;
    byDate.set(d.date, cur);
  }
  return [...byDate.values()].sort((a, b) => a.date.localeCompare(b.date));
}
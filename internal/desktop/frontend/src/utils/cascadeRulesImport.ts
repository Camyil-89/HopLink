/** Normalize one routing rule line (Clash / sing-box style or regex). */
export function normalizeRuleLine(line: string): string {
  let s = line.trim();
  if (s.endsWith(",")) s = s.slice(0, -1).trim();
  if (
    (s.startsWith('"') && s.endsWith('"')) ||
    (s.startsWith("'") && s.endsWith("'"))
  ) {
    s = s.slice(1, -1).trim();
  }
  return s;
}

function normalizeRuleLines(lines: string[]): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const line of lines) {
    const norm = normalizeRuleLine(line);
    if (!norm || norm.startsWith("#")) continue;
    const key = norm.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(norm);
  }
  return out;
}

function splitLooseLines(raw: string): string[] {
  return raw
    .split(/\r?\n/)
    .flatMap((line) => line.split(","))
    .map((part) => part.trim())
    .filter(Boolean);
}

/**
 * Accepts a JSON array ["domain:ru", ...], a JSON object with an array field,
 * or plain text / comma-separated lines.
 */
export function parseRoutingRulesImport(raw: string): string[] {
  const trimmed = raw.trim();
  if (!trimmed) return [];

  if (trimmed.startsWith("[") || trimmed.startsWith("{")) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(trimmed);
    } catch {
      throw new Error("Некорректный JSON");
    }
    if (Array.isArray(parsed)) {
      return normalizeRuleLines(parsed.map(String));
    }
    if (parsed && typeof parsed === "object") {
      const obj = parsed as Record<string, unknown>;
      for (const key of [
        "rules",
        "domains",
        "domain",
        "route",
        "outbound",
        "bypass",
        "direct",
      ]) {
        const val = obj[key];
        if (Array.isArray(val)) {
          return normalizeRuleLines(val.map(String));
        }
      }
    }
    throw new Error("В JSON нужен массив строк или поле с массивом правил");
  }

  return normalizeRuleLines(splitLooseLines(trimmed));
}

export function mergeCascadePatterns(
  existing: string,
  imported: string[],
): string {
  const current = normalizeRuleLines(
    existing.split(/\r?\n/).filter((l) => l.trim()),
  );
  const seen = new Set(current.map((l) => l.toLowerCase()));
  for (const line of imported) {
    const key = line.toLowerCase();
    if (seen.has(key)) continue;
    current.push(line);
    seen.add(key);
  }
  return current.length ? `${current.join("\n")}\n` : "";
}
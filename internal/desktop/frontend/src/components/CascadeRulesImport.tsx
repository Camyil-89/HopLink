import { useState } from "react";
import {
  mergeCascadePatterns,
  parseRoutingRulesImport,
} from "../utils/cascadeRulesImport";

type Target = "domains" | "sni" | "both";

type Props = {
  domainPatterns: string;
  sniPatterns: string;
  onChange: (patch: {
    cascadeDomainPatterns?: string;
    cascadeSNIPatterns?: string;
  }) => void;
};

export function CascadeRulesImport({
  domainPatterns,
  sniPatterns,
  onChange,
}: Props) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const [target, setTarget] = useState<Target>("domains");
  const [replace, setReplace] = useState(false);
  const [error, setError] = useState("");
  const [lastCount, setLastCount] = useState<number | null>(null);

  const apply = () => {
    setError("");
    try {
      const rules = parseRoutingRulesImport(text);
      if (rules.length === 0) {
        setError("Список пуст");
        return;
      }
      const join = (existing: string) =>
        replace ? `${rules.join("\n")}\n` : mergeCascadePatterns(existing, rules);

      if (target === "domains" || target === "both") {
        onChange({ cascadeDomainPatterns: join(domainPatterns) });
      }
      if (target === "sni" || target === "both") {
        onChange({ cascadeSNIPatterns: join(sniPatterns) });
      }
      setLastCount(rules.length);
      setText("");
    } catch (e) {
      setLastCount(null);
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className="cascade-rules-import">
      <button
        type="button"
        className="btn secondary"
        onClick={() => setOpen((v) => !v)}
      >
        {open ? "Скрыть импорт" : "Импорт списка правил…"}
      </button>
      {open && (
        <div className="cascade-rules-import-panel">
          <label className="field">
            JSON-массив или текст
            <textarea
              rows={8}
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={'["domain:ru", "domain:vk.com", "keyword:2gis"]'}
              spellCheck={false}
            />
          </label>
          <div className="cascade-rules-import-options">
            <label className="field inline-field">
              Куда добавить
              <select
                value={target}
                onChange={(e) => setTarget(e.target.value as Target)}
              >
                <option value="domains">Домены</option>
                <option value="sni">SNI TLS</option>
                <option value="both">Домены и SNI</option>
              </select>
            </label>
            <label className="checkbox-field">
              <input
                type="checkbox"
                checked={replace}
                onChange={(e) => setReplace(e.target.checked)}
              />
              Заменить список, а не дополнить
            </label>
          </div>
          {error ? (
            <p className="field-hint import-error">{error}</p>
          ) : lastCount !== null ? (
            <p className="field-hint">
              Добавлено правил: {lastCount}. Сохраните маршрут и примените
              цепочку.
            </p>
          ) : (
            <p className="field-hint">
              Можно вставить полный JSON-массив, объект с полем{" "}
              <code>rules</code> или построчный список. Форматы{" "}
              <code>domain:</code>, <code>keyword:</code>, <code>full:</code> и
              regex поддерживаются на сервере.
            </p>
          )}
          <button type="button" className="btn primary" onClick={apply}>
            Добавить в настройки
          </button>
        </div>
      )}
    </div>
  );
}
import { KeyRound, ListFilter, Radio, ShieldCheck } from "lucide-react";
import { CascadeRulesImport } from "../components/CascadeRulesImport";
import type { AppState } from "../types/app";
import { generatePassword } from "../utils/password";

type Props = {
  state: AppState;
  showSecrets: boolean;
  onChange: (patch: Partial<AppState>) => void;
  onToggleSecrets: () => void;
  onGetJournal: () => void;
};

export function SettingsTab({
  state,
  showSecrets,
  onChange,
  onToggleSecrets,
  onGetJournal,
}: Props) {
  const chainMode = state.chainMode || "udp-relay";

  return (
    <section className="content-page route-settings">
      <div className="settings-grid">
        <div className="table-card settings-card settings-card-wide settings-journal-card">
          <div className="settings-section settings-section-tight">
            <div className="section-icon">
              <Radio size={16} />
            </div>
            <div>
              <b>Журнал сервиса</b>
              <small>
                Записи HopLink с узла маршрута за выбранный период.
              </small>
            </div>
            <button className="btn secondary" onClick={onGetJournal}>
              Получить журнал…
            </button>
          </div>
        </div>

        <div className="table-card settings-card">
          <div className="settings-section">
            <div className="section-icon">
              <Radio size={16} />
            </div>
            <div>
              <b>Сеть и узлы</b>
              <small>Порт, домен входа и поведение серверов на карте.</small>
            </div>
          </div>
          <div className="field-row">
            <label className="field">
              UDP порт
              <input
                className="port-field"
                type="number"
                min="1"
                max="65535"
                value={state.port}
                onChange={(event) =>
                  onChange({ port: Number(event.target.value) })
                }
              />
            </label>
            <label className="field">
              Домен входного узла (необязательно)
              <input
                value={state.entryDomain}
                onChange={(event) =>
                  onChange({ entryDomain: event.target.value })
                }
                placeholder="vpn.example.com"
              />
            </label>
          </div>
          <small className="field-hint settings-inline-hint">
            В ссылку подключения добавляется порт маршрута: {state.port}.
          </small>
          <label className="field">
            Уровень логирования узлов
            <select
              value={state.logLevel || "warn"}
              onChange={(event) =>
                onChange({
                  logLevel: event.target.value as typeof state.logLevel,
                })
              }
            >
              <option value="error">Только ошибки</option>
              <option value="warn">Предупреждения</option>
              <option value="info">Информация</option>
              <option value="debug">Отладка</option>
              <option value="trace">Подробная трассировка</option>
            </select>
            <small className="field-hint">
              Применяется ко всем узлам маршрута при нажатии «Применить».
            </small>
          </label>
          <label className="checkbox-field settings-toggle-field">
            <input
              type="checkbox"
              checked={!!state.disableServerPing}
              onChange={(event) =>
                onChange({ disableServerPing: event.target.checked })
              }
            />
            <span>
              Отключить ICMP ping на серверах маршрута при применении
            </span>
          </label>
          <small className="field-hint">
            На каждом узле цепочки создаётся sysctl-настройка (один раз).
            Снятие сервера с карты или выключение этой опции ответ на ping не
            включает.
          </small>
        </div>

        <div className="table-card settings-card">
          <div className="settings-section">
            <div className="section-icon">
              <Radio size={16} />
            </div>
            <div>
              <b>Hysteria2</b>
              <small>Режим цепочки, маскировка и транспорт между hop.</small>
            </div>
          </div>
          <label className="field">
            Режим цепочки
            <select
              value={chainMode}
              onChange={(event) =>
                onChange({ chainMode: event.target.value as AppState["chainMode"] })
              }
            >
              <option value="udp-relay">
                UDP-реле (сквозное шифрование до выхода)
              </option>
              <option value="hy2-cascade">
                Каскад HY2 (VPN на каждом hop, сертификат на входе)
              </option>
            </select>
            <small className="field-hint">
              В каскадном режиме клиентские ссылки ведут на вход; сертификат и
              пользователи на входном узле.
            </small>
          </label>
          <label className="field">
            SNI
            <input
              value={state.sni}
              onChange={(event) => onChange({ sni: event.target.value })}
              placeholder="Оставьте пустым, чтобы не отправлять"
            />
            <small className="field-hint">
              Пустое значение не попадает в ссылку и не подставляется
              автоматически.
            </small>
          </label>
          <label className="field">
            UDP-маскировка
            <select
              value={state.obfsType || ""}
              onChange={(event) =>
                onChange({
                  obfsType: event.target.value as AppState["obfsType"],
                })
              }
            >
              <option value="">Выключена</option>
              <option value="salamander">Salamander</option>
              <option value="gecko">Gecko (экспериментальная)</option>
            </select>
          </label>
          {!!state.obfsType && (
            <div className="password-row mask-password-row">
              <label className="field">
                Пароль UDP-маскировки
                <input
                  type="password"
                  autoComplete="new-password"
                  value={state.obfsPassword || ""}
                  onChange={(event) =>
                    onChange({ obfsPassword: event.target.value })
                  }
                />
                <small className="field-hint">
                  Тот же пароль у HopLink и у клиента; после смены — снова
                  «Применить».
                </small>
              </label>
              <button
                className="btn secondary generate-password"
                type="button"
                onClick={() => onChange({ obfsPassword: generatePassword(32) })}
              >
                Сгенерировать
              </button>
            </div>
          )}
          <label className="field">
            Транспорт между реле
            <select
              value={state.relayTransport || "udp"}
              disabled={chainMode === "hy2-cascade"}
              onChange={(event) =>
                onChange({
                  relayTransport: event.target
                    .value as AppState["relayTransport"],
                })
              }
            >
              <option value="udp">UDP (по умолчанию)</option>
              <option value="ssh">SSH-туннель (мультиплекс UDP)</option>
              <option value="ws">
                WebSocket over TLS (маршрутный сертификат)
              </option>
            </select>
            <small className="field-hint">
              {chainMode === "hy2-cascade"
                ? "В каскаде между hop — Hysteria2 по UDP."
                : state.relayTransport === "ws"
                  ? `WSS шифруется TLS-сертификатом маршрута; порт транспорта: ${(state.port || 22033) + 1}.`
                  : state.relayTransport === "ssh"
                    ? `QUIC через SSH; порт транспорта: ${(state.port || 22033) + 1}.`
                    : "Прямой UDP между узлами цепочки."}
            </small>
          </label>
        </div>

        {chainMode === "hy2-cascade" && (
          <div className="table-card settings-card settings-card-wide settings-rules-card">
            <div className="settings-section">
              <div className="section-icon">
                <ListFilter size={16} />
              </div>
              <div>
                <b>Правила прямого выхода</b>
                <small>
                  Совпавший с правилом трафик с входа выходит напрямую с
                  входного узла; остальной идёт дальше по цепочке.
                </small>
              </div>
            </div>
            <CascadeRulesImport
              domainPatterns={state.cascadeDomainPatterns || ""}
              sniPatterns={state.cascadeSNIPatterns || ""}
              onChange={onChange}
            />
            <div className="settings-rules-fields">
              <label className="field">
                Домены (по строке)
                <textarea
                  rows={5}
                  value={state.cascadeDomainPatterns || ""}
                  onChange={(event) =>
                    onChange({ cascadeDomainPatterns: event.target.value })
                  }
                  placeholder={"domain:ru\\ndomain:yandex.ru"}
                />
                <small className="field-hint">
                  Regex, <code>domain:имя</code>, <code>keyword:текст</code>,{" "}
                  <code>full:хост</code> (как в sing-box/Clash).
                </small>
              </label>
              <label className="field">
                SNI TLS (по строке)
                <textarea
                  rows={5}
                  value={state.cascadeSNIPatterns || ""}
                  onChange={(event) =>
                    onChange({ cascadeSNIPatterns: event.target.value })
                  }
                  placeholder={"(?i)\\.ru$"}
                />
                <small className="field-hint">
                  SNI из ClientHello для TCP. Пустые списки — весь трафик на
                  следующий hop.
                </small>
              </label>
            </div>
          </div>
        )}

        <div className="table-card settings-card settings-card-wide">
          <div className="settings-section">
            <div className="section-icon">
              <KeyRound size={16} />
            </div>
            <div>
              <b>Ключи и сертификаты</b>
              <small>
                Секреты маршрута в зашифрованных настройках.
              </small>
            </div>
            <button className="text-btn" onClick={onToggleSecrets}>
              {showSecrets ? "Скрыть" : "Показать"}
            </button>
          </div>
          {showSecrets ? (
            <div className="settings-secrets-grid">
              <label className="field">
                Общий ключ цепочки
                <textarea
                  rows={3}
                  value={state.chainKey}
                  onChange={(event) =>
                    onChange({ chainKey: event.target.value })
                  }
                />
              </label>
              <label className="field">
                TLS сертификат
                <textarea
                  rows={5}
                  value={state.certificate}
                  onChange={(event) =>
                    onChange({ certificate: event.target.value })
                  }
                />
              </label>
              <label className="field">
                TLS приватный ключ
                <textarea
                  rows={5}
                  value={state.certificateKey}
                  onChange={(event) =>
                    onChange({ certificateKey: event.target.value })
                  }
                />
              </label>
            </div>
          ) : (
            <div className="secret-state">
              <ShieldCheck size={18} />
              <span>
                <b>
                  {state.chainKey
                    ? "Ключ цепочки сохранён"
                    : "Ключ будет создан при первом применении"}
                </b>
                <small>
                  {state.certificateLifetime ||
                    (state.certificate
                      ? "TLS-сертификат и приватный ключ сохранены"
                      : "TLS-пара будет создана автоматически")}
                </small>
              </span>
            </div>
          )}
          {showSecrets && !!state.certificateLifetime && (
            <p className="field-hint">{state.certificateLifetime}</p>
          )}
        </div>
      </div>
    </section>
  );
}
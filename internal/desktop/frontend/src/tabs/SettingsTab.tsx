import { KeyRound, Radio, ShieldCheck, X } from 'lucide-react'
import type { AppState } from '../types/app'
import { generatePassword } from '../utils/password'

type Props = {
  state: AppState
  showSecrets: boolean
  onChange: (patch: Partial<AppState>) => void
  onToggleSecrets: () => void
  onGetJournal: () => void
}

export function SettingsTab({ state, showSecrets, onChange, onToggleSecrets, onGetJournal }: Props) {
  return (
    <section className="content-page route-settings">
      <div className="settings-grid">
        <div className="table-card settings-card">
          <div className="settings-section"><div className="section-icon"><Radio size={16} /></div><div><b>Журнал сервиса</b><small>Получить записи HopLink за выбранный период с выбранного узла.</small></div></div>
          <button className="btn secondary" onClick={onGetJournal}>Получить журнал…</button>
        </div>
        <div className="table-card settings-card">
          <div className="settings-section">
            <div className="section-icon"><Radio size={16} /></div>
            <div><b>Hysteria2</b><small>Параметры входного и межсерверных подключений.</small></div>
          </div>
          <label className="field">SNI<input value={state.sni} onChange={event => onChange({ sni: event.target.value })} placeholder="Оставьте пустым, чтобы не отправлять" /><small className="field-hint">Пустое значение не добавляется в ссылку и не подставляется автоматически.</small></label>
          <label className="field">UDP-маскировка
            <select value={state.obfsType || ''} onChange={event => onChange({ obfsType: event.target.value as AppState['obfsType'], obfsPassword: event.target.value ? state.obfsPassword : '' })}>
              <option value="">Выключена</option><option value="salamander">Salamander</option><option value="gecko">Gecko (экспериментальная)</option>
            </select>
          </label>
          {!!state.obfsType && <div className="password-row mask-password-row">
            <label className="field">Пароль UDP-маскировки
              <input type="password" autoComplete="new-password" value={state.obfsPassword || ''} onChange={event => onChange({ obfsPassword: event.target.value })} />
              <small className="field-hint">Один и тот же пароль используется HopLink и клиентом. При смене режима примените маршрут заново.</small>
            </label>
            <button className="btn secondary generate-password" type="button" onClick={() => onChange({ obfsPassword: generatePassword(32) })}>Сгенерировать</button>
          </div>}
          <label className="field">Транспорт между реле
            <select value={state.relayTransport || 'udp'} onChange={event => onChange({ relayTransport: event.target.value as AppState['relayTransport'] })}>
              <option value="udp">UDP (по умолчанию)</option>
              <option value="ssh">SSH-туннель (мультиплекс UDP)</option>
              <option value="ws">WebSocket over TLS (маршрутный сертификат)</option>
            </select>
            <small className="field-hint">
              {state.relayTransport === 'ws'
                ? `WSS между реле шифруется основным TLS-сертификатом маршрута. Порт транспорта: ${(state.port || 22033) + 1}.`
                : state.relayTransport === 'ssh'
                  ? `SSH режим передаёт QUIC-пакеты между узлами через зашифрованный канал; порт SSH-транспорта: ${(state.port || 22033) + 1}.`
                  : 'Прямая передача UDP между узлами цепочки.'}
            </small>
          </label>
          <label className="field">UDP порт<input type="number" min="1" max="65535" value={state.port} onChange={event => onChange({ port: Number(event.target.value) })} /></label>
          <label className="field">
            Домен входного узла (необязательно)
            <input value={state.entryDomain} onChange={event => onChange({ entryDomain: event.target.value })} placeholder="vpn.example.com" />
            <small className="field-hint">В ссылку подключения будет добавлен порт маршрута: {state.port}.</small>
          </label>
          <label className="field">Уровень логирования узлов
            <select value={state.logLevel || 'warn'} onChange={event => onChange({ logLevel: event.target.value as typeof state.logLevel })}>
              <option value="error">Только ошибки</option><option value="warn">Предупреждения</option><option value="info">Информация</option><option value="debug">Отладка</option><option value="trace">Подробная трассировка</option>
            </select>
            <small className="field-hint">Уровень применяется ко всем узлам маршрута при нажатии «Применить».</small>
          </label>
        </div>

        <div className="table-card settings-card">
          <div className="settings-section">
            <div className="section-icon"><KeyRound size={16} /></div>
            <div><b>Ключи и сертификаты</b><small>Секреты хранятся в зашифрованных настройках выбранного маршрута.</small></div>
            <button className="text-btn" onClick={onToggleSecrets}>{showSecrets ? 'Скрыть' : 'Показать'}</button>
          </div>
          {showSecrets ? (
            <>
              <label className="field">Общий ключ цепочки<textarea rows={3} value={state.chainKey} onChange={event => onChange({ chainKey: event.target.value })} /></label>
              <label className="field">TLS сертификат<textarea rows={5} value={state.certificate} onChange={event => onChange({ certificate: event.target.value })} /></label>
              <label className="field">TLS приватный ключ<textarea rows={5} value={state.certificateKey} onChange={event => onChange({ certificateKey: event.target.value })} /></label>
              {!!state.certificateLifetime && <p className="field-hint">{state.certificateLifetime}</p>}
            </>
          ) : (
            <div className="secret-state">
              <ShieldCheck size={18} />
              <span>
                <b>{state.chainKey ? 'Ключ цепочки сохранён' : 'Ключ будет создан при первом применении'}</b>
                <small>{state.certificateLifetime || (state.certificate ? 'TLS-сертификат и приватный ключ сохранены' : 'TLS-пара будет создана автоматически')}</small>
              </span>
            </div>
          )}
        </div>

        <div className="table-card settings-card">
          <div className="settings-section">
            <div className="section-icon"><ShieldCheck size={16} /></div>
            <div><b>Доверенные SSH ключи</b><small>Закреплённые fingerprints серверов.</small></div>
          </div>
          {Object.entries(state.knownHosts || {}).length ? Object.entries(state.knownHosts).map(([host, fingerprint]) => (
            <div className="trust-row" key={host}>
              <span>{host}</span><code>{fingerprint}</code>
              <button className="icon-btn danger" title="Удалить доверенный ключ" onClick={() => {
                const knownHosts = { ...state.knownHosts }
                delete knownHosts[host]
                onChange({ knownHosts })
              }}><X size={14} /></button>
            </div>
          )) : (
            <div className="secret-state"><span className="muted">Fingerprint будет запрошен при первом SSH-подключении.</span></div>
          )}
        </div>
      </div>
    </section>
  )
}

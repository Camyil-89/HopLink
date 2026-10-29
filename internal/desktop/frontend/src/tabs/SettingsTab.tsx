import { KeyRound, Radio, ShieldCheck, X } from 'lucide-react'
import type { AppState } from '../types/app'

type Props = {
  state: AppState
  showSecrets: boolean
  onChange: (patch: Partial<AppState>) => void
  onToggleSecrets: () => void
}

export function SettingsTab({ state, showSecrets, onChange, onToggleSecrets }: Props) {
  return (
    <section className="content-page route-settings">
      <div className="settings-grid">
        <div className="table-card settings-card">
          <div className="settings-section">
            <div className="section-icon"><Radio size={16} /></div>
            <div><b>Hysteria2</b><small>Параметры входного и межсерверных подключений.</small></div>
          </div>
          <label className="field">SNI<input value={state.sni} onChange={event => onChange({ sni: event.target.value })} /></label>
          <label className="field">UDP порт<input type="number" min="1" max="65535" value={state.port} onChange={event => onChange({ port: Number(event.target.value) })} /></label>
          <label className="field">
            Домен входного узла (необязательно)
            <input value={state.entryDomain} onChange={event => onChange({ entryDomain: event.target.value })} placeholder="vpn.example.com" />
            <small className="field-hint">В ссылку подключения будет добавлен порт маршрута: {state.port}.</small>
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
            </>
          ) : (
            <div className="secret-state">
              <ShieldCheck size={18} />
              <span>
                <b>{state.chainKey ? 'Ключ цепочки сохранён' : 'Ключ будет создан при первом применении'}</b>
                <small>{state.certificate ? 'TLS-сертификат и приватный ключ сохранены' : 'TLS-пара будет создана автоматически'}</small>
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

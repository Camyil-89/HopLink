import { useState } from 'react'
import type { Server } from '../types/app'

type Props = { servers: Server[]; onClose: () => void; onLoad: (serverID: string, since: string, until: string) => Promise<string> }
const localDateTime = (date: Date) => { const copy = new Date(date.getTime() - date.getTimezoneOffset() * 60000); return copy.toISOString().slice(0, 16) }

export function RouteJournalModal({ servers, onClose, onLoad }: Props) {
  const [serverID, setServerID] = useState(servers[0]?.id || '')
  const [since, setSince] = useState(localDateTime(new Date(Date.now() - 60 * 60 * 1000)))
  const [until, setUntil] = useState(localDateTime(new Date()))
  const [output, setOutput] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const load = async () => { setBusy(true); setError(''); try { setOutput(await onLoad(serverID, new Date(since).toISOString(), new Date(until).toISOString())) } catch (e: any) { setError(String(e)) } finally { setBusy(false) } }
  return <div className="modal-backdrop"><section className="modal journal-modal" role="dialog" aria-modal="true">
    <div className="modal-head"><div><h2>Журнал сервиса маршрута</h2><p>Выберите сервер и период. Используется часовой пояс компьютера.</p></div><button className="icon-btn" onClick={onClose}>×</button></div>
    <label className="field">Сервер<select value={serverID} onChange={e => setServerID(e.target.value)}>{servers.map(s => <option key={s.id} value={s.id}>{s.name || s.host} ({s.host})</option>)}</select></label>
    <div className="field-row"><label className="field">С начала<input type="datetime-local" value={since} onChange={e => setSince(e.target.value)} /></label><label className="field">По<input type="datetime-local" value={until} onChange={e => setUntil(e.target.value)} /></label></div>
    <div className="modal-actions"><button className="btn" onClick={onClose}>Закрыть</button><button className="btn primary" disabled={busy || !serverID || !since || !until} onClick={() => void load()}>{busy ? 'Загружаю…' : 'Получить журнал'}</button></div>
    {error && <p className="journal-error">{error}</p>}{output && <pre className="journal-output">{output}</pre>}
  </section></div>
}

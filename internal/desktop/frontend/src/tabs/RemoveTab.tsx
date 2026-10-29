import { LockKeyhole, Server as ServerIcon, Trash2 } from 'lucide-react'
import type { AppState } from '../types/app'

type Props = { state: AppState; selected: string[]; busy: boolean; onToggle: (id: string) => void; onToggleAll: () => void; onRemove: () => void }

export function RemoveTab({ state, selected, busy, onToggle, onToggleAll, onRemove }: Props) {
  return (
    <section className="content-page">
      <div className="page-head">
        <div><div className="eyebrow">ОБСЛУЖИВАНИЕ <span>•</span> SSH</div><h1>Удалить HopLink</h1><p>Остановить сервисы всех маршрутов и удалить их файлы на выбранных серверах.</p></div>
        <button className="btn danger-btn" onClick={onRemove} disabled={!selected.length || busy}><Trash2 size={16} /> Удалить на выбранных ({selected.length})</button>
      </div>
      <div className="table-card">
        <div className="table-caption"><div><b>Серверы</b><small>Выберите один или несколько серверов для очистки.</small></div><button className="text-btn" onClick={onToggleAll}>{selected.length === state.servers.length ? 'Снять выбор' : 'Выбрать все'}</button></div>
        <table>
          <thead><tr><th /><th>СЕРВЕР</th><th>SSH АДРЕС</th><th>РОЛЬ</th><th>ОЧИСТКА</th></tr></thead>
          <tbody>{state.servers.map(server => <tr key={server.id}>
            <td><input type="checkbox" checked={selected.includes(server.id)} onChange={() => onToggle(server.id)} /></td>
            <td><div className="user-cell"><div className="server-symbol"><ServerIcon size={15} /></div><b>{server.name}</b></div></td>
            <td>{server.sshUser}@{server.host}:{server.sshPort}</td>
            <td>{server.role === 'entry' ? 'Вход' : server.role === 'exit' ? 'Выход' : '—'}</td>
            <td><span className="status-pill neutral">Все маршруты</span></td>
          </tr>)}</tbody>
        </table>
      </div>
      <div className="notice-card"><LockKeyhole size={17} /><span>Будут удалены все маршруты на выбранных серверах. Сохранённые карты останутся без изменений.</span></div>
    </section>
  )
}

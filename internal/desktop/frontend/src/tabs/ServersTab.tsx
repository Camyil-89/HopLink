import { Plus, Pencil, Server as ServerIcon, Trash2 } from 'lucide-react'
import type { AppState, Server } from '../types/app'

type Props = {
  state: AppState
  busy: boolean
  onAdd: () => void
  onEdit: (server: Server) => void
  onDelete: (id: string) => void
}

export function ServersTab({ state, busy, onAdd, onEdit, onDelete }: Props) {
  return (
    <section className="content-page">
      <div className="page-head">
        <div><div className="eyebrow">ИНФРАСТРУКТУРА <span>•</span> {state.servers.length} СЕРВЕРОВ</div><h1>Серверы</h1><p>Добавляйте SSH-доступы здесь, затем размещайте серверы на карте цепочки.</p></div>
        <button className="btn primary" onClick={onAdd}><Plus size={16} /> Добавить сервер</button>
      </div>
      <div className="table-card">
        <div className="table-caption"><div><b>Библиотека серверов</b><small>Реквизиты хранятся локально в настройках приложения.</small></div><span className="status-pill neutral">{state.servers.length} записей</span></div>
        {state.servers.length ? <table>
          <thead><tr><th>СЕРВЕР</th><th>SSH АДРЕС</th><th>КАРТА / ДЕПЛОЙ</th><th>АУТЕНТИФИКАЦИЯ</th><th /></tr></thead>
          <tbody>{state.servers.map(server => {
            const onMap = state.mapServers.some(mapped => mapped.id === server.id)
            const deployed = state.deployedServers?.some(item => item.id === server.id)
            return <tr key={server.id}>
              <td><div className="user-cell"><div className="server-symbol"><ServerIcon size={15} /></div><b>{server.name}</b></div></td>
              <td>{server.host}:{server.sshPort}<small className="server-user">{server.sshUser}</small></td>
              <td><span className={`status-pill ${onMap || deployed ? '' : 'neutral'}`}><i />{deployed ? 'HopLink установлен' : onMap ? 'На карте' : 'В библиотеке'}</span></td>
              <td>{server.sshPrivateKey ? 'SSH ключ' : 'SSH пароль'}</td>
              <td><div className="row-actions">
                <button className="icon-btn" onClick={() => onEdit(server)} title="Изменить"><Pencil size={14} /></button>
                <button className="icon-btn danger" disabled={busy} onClick={() => onDelete(server.id)} title="Удалить сервер и установленный HopLink"><Trash2 size={15} /></button>
              </div></td>
            </tr>
          })}</tbody>
        </table> : <div className="empty-table"><ServerIcon size={28} /><b>Серверов пока нет</b><span>Добавьте SSH-доступы, чтобы построить цепочку.</span><button className="btn primary" onClick={onAdd}><Plus size={15} /> Добавить сервер</button></div>}
      </div>
    </section>
  )
}

import { Copy, Pencil, Plus, Trash2, Users } from 'lucide-react'
import type { AppState, User } from '../types/app'

type Props = {
  state: AppState
  onAdd: () => void
  onEdit: (user: User) => void
  onDelete: (id: string) => void
  onCopy: (user: User) => void
  onToggle: (id: string, active: boolean) => void
}

export function UsersTab({ state, onAdd, onEdit, onDelete, onCopy, onToggle }: Props) {
  return (
    <section className="content-page route-users">
      <div className="page-head">
        <div><div className="eyebrow">ПОЛЬЗОВАТЕЛИ МАРШРУТА <span>•</span> {state.users.length}</div><h1>Пользователи маршрута</h1><p>Учётные данные этого маршрута не используются в других маршрутах.</p></div>
        <button className="btn primary" onClick={onAdd}><Plus size={16} /> Добавить пользователя</button>
      </div>
      <div className="table-card">
        <div className="table-caption"><div><b>Клиенты Hiddify</b><small>Логины, пароли и ссылки подключения хранятся в настройках выбранного маршрута.</small></div></div>
        {state.users.length ? <table>
          <thead><tr><th>ИМЯ</th><th>ПАРОЛЬ</th><th>ССЫЛКА ПОДКЛЮЧЕНИЯ</th><th>СТАТУС</th><th /></tr></thead>
          <tbody>{state.users.map(user => { const active = user.active !== false; return <tr key={user.id}>
            <td><div className="user-cell"><div className="user-avatar">{user.name.slice(0, 1).toUpperCase()}</div><b>{user.name}</b></div></td>
            <td><span className="masked">{user.password}</span></td>
            <td className="user-link-cell">{active && user.link ? <><code title={user.link}>{user.link}</code><button className="icon-btn" title="Скопировать ссылку" onClick={() => onCopy(user)}><Copy size={14} /></button></> : <span className="muted">{active ? 'Будет записана после применения маршрута' : 'Недоступна, пока пользователь отключён'}</span>}</td>
            <td><button className={`status-pill user-status ${active ? 'active' : 'inactive'}`} onClick={() => onToggle(user.id, !active)} title={active ? 'Нажмите, чтобы отключить пользователя' : 'Нажмите, чтобы активировать пользователя'}><i /> {active ? 'Активен' : 'Отключён'}</button></td>
            <td><div className="row-actions"><button className="icon-btn" onClick={() => onEdit(user)} title="Изменить"><Pencil size={14} /></button><button className="icon-btn danger" onClick={() => onDelete(user.id)}><Trash2 size={15} /></button></div></td>
          </tr>})}</tbody>
        </table> : <div className="empty-table"><Users size={28} /><b>Пока нет пользователей</b><span>Добавьте учётную запись для подключения Hiddify.</span></div>}
      </div>
    </section>
  )
}

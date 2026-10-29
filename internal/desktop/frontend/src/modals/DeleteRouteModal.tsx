import { Trash2, X } from 'lucide-react'
import type { Route, Server } from '../types/app'

type Props = { route: Route; onCancel: () => void; onConfirm: () => void }

export function DeleteRouteModal({ route, onCancel, onConfirm }: Props) {
  const servers: Server[] = route.deployedServers || []
  return (
    <div className="modal-backdrop">
      <div className="modal compact" role="dialog" aria-modal="true" aria-labelledby="delete-route-title" onClick={event => event.stopPropagation()}>
        <div className="modal-head">
          <div><h2 id="delete-route-title">Удалить сервис и маршрут?</h2><p>Маршрут «{route.name}»</p></div>
          <button className="icon-btn" onClick={onCancel} title="Закрыть"><X size={17} /></button>
        </div>
        <div className="cancel-warning">{servers.length > 0 ? `После подтверждения HopLink будет остановлен и удалён с ${servers.length} серверов маршрута. Затем будут удалены его настройки, карта, пользователи и ссылки.` : 'У этого маршрута нет зарегистрированных установок. Его настройки, карта, пользователи и ссылки будут удалены.'} Если удалить сервис не удастся, данные маршрута сохранятся, чтобы можно было повторить удаление. Это действие необратимо.</div>
        {!!servers.length && <div className="route-remove-server-list">{servers.map(server => <span key={server.id}>{server.name || server.host} <small>{server.host}:{server.sshPort}</small></span>)}</div>}
        <div className="modal-actions">
          <button className="btn secondary" onClick={onCancel}>Отмена</button>
          <button className="btn danger-btn" onClick={onConfirm}><Trash2 size={15} /> Удалить сервис и маршрут</button>
        </div>
      </div>
    </div>
  )
}

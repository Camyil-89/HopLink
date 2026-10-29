import { Trash2, X } from 'lucide-react'
import type { Server } from '../types/app'

type Props = {
  routeName: string
  servers: Server[]
  onCancel: () => void
  onConfirm: () => void
}

export function RemoveRouteServiceModal({ routeName, servers, onCancel, onConfirm }: Props) {
  return (
    <div className="modal-backdrop">
      <div className="modal compact" role="dialog" aria-modal="true" aria-labelledby="remove-route-service-title" onClick={event => event.stopPropagation()}>
        <div className="modal-head">
          <div>
            <h2 id="remove-route-service-title">Удалить сервис маршрута?</h2>
            <p>Маршрут «{routeName}» · {servers.length} серверов</p>
          </div>
          <button className="icon-btn" onClick={onCancel} title="Закрыть"><X size={17} /></button>
        </div>
        <div className="cancel-warning">HopLink этого маршрута будет остановлен и удалён с серверов. Карта, настройки и пользователи сохранятся.</div>
        <div className="route-remove-server-list">{servers.map(server => <span key={server.id}>{server.name || server.host} <small>{server.host}:{server.sshPort}</small></span>)}</div>
        <div className="modal-actions">
          <button className="btn secondary" onClick={onCancel}>Отмена</button>
          <button className="btn danger-btn" onClick={onConfirm}><Trash2 size={15} /> Удалить сервис</button>
        </div>
      </div>
    </div>
  )
}

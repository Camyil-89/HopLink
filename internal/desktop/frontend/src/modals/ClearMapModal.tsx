type Props = {
  serverCount: number
  connectionCount: number
  onCancel: () => void
  onConfirm: () => void
}

export function ClearMapModal({ serverCount, connectionCount, onCancel, onConfirm }: Props) {
  return (
    <div className="modal-backdrop" onClick={onCancel}>
      <div className="modal compact" role="dialog" aria-modal="true" aria-labelledby="clear-map-title" onClick={event => event.stopPropagation()}>
        <div className="modal-head">
          <div><h2 id="clear-map-title">Очистить карту маршрута?</h2><p>Будут убраны {serverCount} серверов и {connectionCount} связей.</p></div>
        </div>
        <div className="cancel-warning">Установленный сервис изменится только после нажатия «Применить маршрут».</div>
        <div className="modal-actions">
          <button className="btn secondary" onClick={onCancel}>Отмена</button>
          <button className="btn danger-btn" onClick={onConfirm}>Очистить карту</button>
        </div>
      </div>
    </div>
  )
}

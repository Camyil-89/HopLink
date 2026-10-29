import { Plus, X } from 'lucide-react'

type Props = { name: string; onNameChange: (name: string) => void; onClose: () => void; onCreate: () => void }

export function RouteModal({ name, onNameChange, onClose, onCreate }: Props) {
  return (
    <div className="modal-backdrop">
      <form className="modal compact" onSubmit={event => { event.preventDefault(); onCreate() }} onClick={event => event.stopPropagation()}>
        <div className="modal-head">
          <div><h2>Новый маршрут</h2><p>У маршрута будут отдельные карта, параметры и сертификаты.</p></div>
          <button type="button" className="icon-btn" onClick={onClose}><X size={17} /></button>
        </div>
        <label className="field">Название<input autoFocus value={name} onChange={event => onNameChange(event.target.value)} placeholder="Например, Европа" required /></label>
        <div className="modal-actions"><button type="button" className="btn secondary" onClick={onClose}>Отмена</button><button className="btn primary" type="submit"><Plus size={15} /> Создать маршрут</button></div>
      </form>
    </div>
  )
}

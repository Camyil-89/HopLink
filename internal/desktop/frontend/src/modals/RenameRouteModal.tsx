import { Check, X } from 'lucide-react'

type Props = { name: string; onNameChange: (name: string) => void; onClose: () => void; onSave: () => void }

export function RenameRouteModal({ name, onNameChange, onClose, onSave }: Props) {
  return (
    <div className="modal-backdrop">
      <form className="modal compact" onSubmit={event => { event.preventDefault(); onSave() }}>
        <div className="modal-head">
          <div><h2>Переименовать маршрут</h2><p>Новое имя будет видно в списке и настройках маршрута.</p></div>
          <button type="button" className="icon-btn" onClick={onClose} title="Закрыть"><X size={17} /></button>
        </div>
        <label className="field">Название маршрута<input autoFocus value={name} onChange={event => onNameChange(event.target.value)} /></label>
        <div className="modal-actions"><button type="button" className="btn secondary" onClick={onClose}>Отмена</button><button className="btn primary" type="submit"><Check size={15} /> Сохранить</button></div>
      </form>
    </div>
  )
}

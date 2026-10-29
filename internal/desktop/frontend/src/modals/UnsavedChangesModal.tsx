import { Check, ShieldCheck } from 'lucide-react'

type Props = { onStay: () => void; onDiscard: () => void; onSave: () => void; saving?: boolean }

export function UnsavedChangesModal({ onStay, onDiscard, onSave, saving = false }: Props) {
  return (
    <div className="modal-backdrop unsaved-backdrop">
      <div className="modal compact" role="dialog" aria-modal="true" aria-labelledby="unsaved-title" onClick={event => event.stopPropagation()}>
        <div className="modal-head">
          <div><h2 id="unsaved-title">Есть несохранённые изменения</h2><p>Изменения в карте или настройках маршрута ещё не сохранены.</p></div>
          <ShieldCheck size={21} className="trust-icon" />
        </div>
        <div className="cancel-warning">Сохраните их, чтобы они остались в маршруте. Если продолжить без сохранения, изменения будут отброшены.</div>
        <div className="modal-actions">
          <button className="btn secondary" onClick={onStay} disabled={saving}>Остаться</button>
          <button className="btn danger-btn" onClick={onDiscard} disabled={saving}>Выйти без сохранения</button>
          <button className="btn primary" onClick={onSave} disabled={saving}><Check size={14} /> {saving ? 'Сохранение…' : 'Сохранить и продолжить'}</button>
        </div>
      </div>
    </div>
  )
}

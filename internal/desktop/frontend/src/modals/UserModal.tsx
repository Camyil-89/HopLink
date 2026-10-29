import { Check, KeyRound, Plus, X } from 'lucide-react'

export type UserDraft = { name: string; password: string }
type Props = { draft: UserDraft; editing: boolean; onChange: (draft: UserDraft) => void; onGeneratePassword: () => void; onClose: () => void; onSave: () => void }

export function UserModal({ draft, editing, onChange, onGeneratePassword, onClose, onSave }: Props) {
  return (
    <div className="modal-backdrop">
      <div className="modal compact" onClick={event => event.stopPropagation()}>
        <div className="modal-head"><div><h2>{editing ? 'Изменить пользователя' : 'Новый пользователь'}</h2><p>Логин и пароль для подключения Hiddify</p></div><button className="icon-btn" onClick={onClose}><X size={17} /></button></div>
        <div className="modal-fields">
          <label className="field">Логин<input value={draft.name} onChange={event => onChange({ ...draft, name: event.target.value })} placeholder="user_name" /></label>
          <div className="field-row password-row"><label className="field">Пароль<input value={draft.password} onChange={event => onChange({ ...draft, password: event.target.value })} placeholder="Уникальный пароль" /></label><button type="button" className="btn secondary generate-password" onClick={onGeneratePassword}><KeyRound size={14} /> Сгенерировать</button></div>
        </div>
        <div className="modal-actions"><button className="btn secondary" onClick={onClose}>Отмена</button><button className="btn primary" onClick={onSave}><Check size={15} /> {editing ? 'Сохранить' : 'Добавить'}</button></div>
      </div>
    </div>
  )
}

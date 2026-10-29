import { Check, LockKeyhole, Plus, X } from 'lucide-react'

export type ServerDraft = { name: string; host: string; sshPort: number; sshUser: string; sshPassword: string; sshPrivateKey: string; sshKeyPassphrase: string }
type Props = { draft: ServerDraft; editing: boolean; onChange: (draft: ServerDraft) => void; onClose: () => void; onSave: () => void }

export function ServerModal({ draft, editing, onChange, onClose, onSave }: Props) {
  const update = (patch: Partial<ServerDraft>) => onChange({ ...draft, ...patch })
  return (
    <div className="modal-backdrop">
      <div className="modal" onClick={event => event.stopPropagation()}>
        <div className="modal-head"><div><h2>{editing ? 'Изменить сервер' : 'Добавить сервер'}</h2><p>SSH доступ к Linux-серверу</p></div><button className="icon-btn" onClick={onClose}><X size={17} /></button></div>
        <div className="modal-fields">
          <label className="field">Название<input placeholder="Например, Frankfurt-1" value={draft.name} onChange={event => update({ name: event.target.value })} /></label>
          <div className="field-row"><label className="field">Адрес / IP<input placeholder="203.0.113.10" value={draft.host} onChange={event => update({ host: event.target.value })} /></label><label className="field port-field">SSH порт<input type="number" value={draft.sshPort} onChange={event => update({ sshPort: Number(event.target.value) })} /></label></div>
          <label className="field">SSH пользователь<input value={draft.sshUser} onChange={event => update({ sshUser: event.target.value })} /></label>
          <label className="field">SSH пароль (если используется)<input type="password" value={draft.sshPassword} onChange={event => update({ sshPassword: event.target.value })} /></label>
          <label className="field">SSH приватный ключ (необязательно)<textarea rows={4} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" value={draft.sshPrivateKey} onChange={event => update({ sshPrivateKey: event.target.value })} /></label>
          <label className="field">Пароль приватного ключа<input type="password" value={draft.sshKeyPassphrase} onChange={event => update({ sshKeyPassphrase: event.target.value })} /></label>
          <div className="inline-secure"><LockKeyhole size={14} /> Укажите SSH пароль или приватный ключ. Реквизиты сохраняются в зашифрованных настройках приложения.</div>
        </div>
        <div className="modal-actions"><button className="btn secondary" onClick={onClose}>Отмена</button><button className="btn primary" onClick={onSave}>{editing ? <Check size={15} /> : <Plus size={15} />} {editing ? 'Сохранить' : 'Добавить сервер'}</button></div>
      </div>
    </div>
  )
}

import type { RefObject } from 'react'
import { Check, ShieldCheck, X } from 'lucide-react'

type ProgressProps = {
  title: string
  status: string
  logs: string[]
  busy: boolean
  result: 'success' | 'error' | 'cancelled' | null
  cancelable: boolean
  endRef: RefObject<HTMLSpanElement>
  onClose: () => void
  onCancel: () => void
}

export function ProgressModal({ title, status, logs, busy, result, cancelable, endRef, onClose, onCancel }: ProgressProps) {
  const state = busy ? 'running' : result || 'error'
  return (
    <div className="modal-backdrop progress-backdrop">
      <div className="modal progress-modal" role="dialog" aria-modal="true" aria-label={title} onClick={event => event.stopPropagation()}>
        <div className="modal-head"><div><h2>{title}</h2></div><button className="icon-btn" onClick={onClose} disabled={busy && !cancelable} title={busy ? (cancelable ? 'Запросить отмену' : 'Операция выполняется') : 'Закрыть'}><X size={17} /></button></div>
        <div className={`progress-summary ${state}`} role="status"><span className="progress-summary-mark">{state === 'running' ? '…' : state === 'success' ? '✓' : '!'}</span><div><strong>{state === 'running' ? 'Выполняется' : state === 'success' ? 'Готово' : state === 'cancelled' ? 'Отменено' : 'Ошибка'}</strong><small>{status}</small></div></div>
        <pre className="progress-log" role="log" aria-live="polite">{logs.length ? logs.join('\n') : 'Ожидание этапов синхронизации…'}<span ref={endRef} /></pre>
        <div className="progress-actions"><span className="muted">{logs.length} записей</span>{busy ? (cancelable ? <button className="btn danger-btn" onClick={onCancel}>Отменить</button> : <span className="muted">Дождитесь завершения</span>) : <button className="btn primary" onClick={onClose}>Закрыть</button>}</div>
      </div>
    </div>
  )
}

type HostApprovalProps = { host: string; fingerprint: string; onCancel: () => void; onApprove: () => void }

export function HostApprovalModal({ host, fingerprint, onCancel, onApprove }: HostApprovalProps) {
  return (
    <div className="modal-backdrop"><div className="modal compact">
      <div className="modal-head"><div><h2>Подтвердить SSH-сервер</h2><p>Новый ключ хоста для {host}</p></div><ShieldCheck size={22} className="trust-icon" /></div>
      <div className="fingerprint-box"><small>SHA256 FINGERPRINT</small><code>{fingerprint}</code><p>Сверьте fingerprint с консолью или панелью вашего сервера. После подтверждения он будет сохранён локально.</p></div>
      <div className="modal-actions"><button className="btn secondary" onClick={onCancel}>Отмена</button><button className="btn primary" onClick={onApprove}><Check size={15} /> Доверять этому серверу</button></div>
    </div></div>
  )
}

type CancelProps = { onContinue: () => void; onConfirm: () => void }

export function CancelApplyModal({ onContinue, onConfirm }: CancelProps) {
  return (
    <div className="modal-backdrop cancel-confirm-backdrop"><div className="modal compact" onClick={event => event.stopPropagation()}>
      <div className="modal-head"><div><h2>Отменить синхронизацию?</h2><p>Операция остановится после закрытия текущего SSH-действия.</p></div><ShieldCheck size={21} className="trust-icon" /></div>
      <div className="cancel-warning">На серверах могут остаться частично применённые изменения. После отмены проверьте цепочку и запустите синхронизацию повторно.</div>
      <div className="modal-actions"><button className="btn secondary" onClick={onContinue}>Продолжить</button><button className="btn danger-btn" onClick={onConfirm}>Да, отменить</button></div>
    </div></div>
  )
}

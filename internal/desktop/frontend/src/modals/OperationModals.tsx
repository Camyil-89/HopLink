import type { CSSProperties, RefObject } from 'react'
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
  const stage = Math.min(4, Math.max(0, logs.reduce((current, line) => {
    if (/Проверяю SSH|Проверяю SSH-доступ|SSH подключён|Сервер готов/.test(line)) return Math.max(current, 1)
    if (/Собираю|Скачиваю/.test(line)) return Math.max(current, 2)
    if (/Загружаю файлы|Формирую конфигурацию|Устанавливаю HopLink|Устанавливаю бинарник|Останавливаю текущие службы/.test(line)) return Math.max(current, 3)
    if (/Запускаю цепочку|Проверяю запуск|Проверяю состояние всех служб/.test(line)) return Math.max(current, 4)
    return current
  }, 0)))
  const stages = ['Связь', 'Подготовка', 'Установка', 'Запуск']
  return (
    <div className="modal-backdrop progress-backdrop">
      <div className="modal progress-modal" role="dialog" aria-modal="true" aria-label={title} onClick={event => event.stopPropagation()}>
        <div className="modal-head"><div><h2>{title}</h2></div><button className="icon-btn" onClick={onClose} disabled={busy && !cancelable} title={busy ? (cancelable ? 'Запросить отмену' : 'Операция выполняется') : 'Закрыть'}><X size={17} /></button></div>
        <div className={`sync-visual ${state}`} style={{ '--sync-progress': `${state === 'success' ? 100 : Math.max(0, stage - 1) * 100 / 3}%` } as CSSProperties} aria-hidden="true">
          <div className="sync-visual-orbit orbit-one" /><div className="sync-visual-orbit orbit-two" />
          <div className="sync-visual-core"><span className="sync-core-symbol">{state === 'success' ? '✓' : state === 'error' ? '!' : state === 'cancelled' ? 'Ⅱ' : '↗'}</span><span className="sync-core-caption">HOPLINK</span></div>
          <div className="sync-route">
            <div className="sync-track"><i /></div><span className="sync-packet" />
            {[1, 2, 3, 4].map((step, index) => <span key={step} className={`sync-node node-${step} ${stage >= step + 1 || state === 'success' ? 'reached' : ''}`} style={{ left: `${index * 100 / 3}%` }} />)}
          </div>
          <div className="sync-stage-labels">{stages.map((item, index) => <span key={item} className={stage >= index + 1 || state === 'success' ? 'reached' : ''}>{item}</span>)}</div>
        </div>
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

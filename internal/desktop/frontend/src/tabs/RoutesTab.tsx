import { ArrowRight, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import type { Route, RouteServiceStatus } from '../types/app'

type Props = { routes: Route[]; statuses: Record<string, RouteServiceStatus>; checking: boolean; onCheck: () => void; onSelect: (id: string) => void; onCreate: () => void; onRename: (route: Route) => void; onDelete: (route: Route) => void }

export function RoutesTab({ routes, statuses, checking, onCheck, onSelect, onCreate, onRename, onDelete }: Props) {
  return (
    <section className="route-landing">
      <div className="route-landing-card">
        <div className="eyebrow">МАРШРУТЫ</div>
        <div className="route-landing-heading"><h1>Ваши маршруты</h1><button className="btn secondary" onClick={onCheck} disabled={checking || !routes.length}><RefreshCw size={14} className={checking ? 'spin' : ''} /> {checking ? 'Проверка…' : 'Проверить'}</button></div>
        <p>Выберите маршрут, чтобы открыть его карту и настройки.</p>
        <div className="route-list">
          {!(routes ?? []).length && <div className="route-empty">Маршрутов пока нет. Создайте маршрут, чтобы начать настройку.</div>}
          {(routes ?? []).map(route => (
            <div key={route.id} className="route-option-row">
              <button className="route-option" onClick={() => onSelect(route.id)}>
                <span><b>{route.name}</b><small>{route.mapServers?.length ?? 0} серверов · UDP {route.port}</small>{statuses[route.id] && <small className={`route-entry-ping ${statuses[route.id].entryLatencyMs !== undefined ? 'measured' : 'unavailable'}`}>Пинг от вас до входа: {statuses[route.id].entryLatencyMs !== undefined ? `${statuses[route.id].entryLatencyMs!.toFixed(1)} мс` : 'ICMP недоступен'}</small>}</span>
                <span className="route-option-end">{statuses[route.id] && <span className={`route-status ${statuses[route.id].running === statuses[route.id].total && statuses[route.id].total > 0 ? 'running' : 'stopped'}`}>{statuses[route.id].running}/{statuses[route.id].total} работает</span>}<ArrowRight size={16} /></span>
              </button>
              <button className="icon-btn route-rename-btn" title={`Переименовать маршрут «${route.name}»`} aria-label={`Переименовать маршрут «${route.name}»`} onClick={() => onRename(route)}><Pencil size={14} /></button>
              <button className="icon-btn danger route-delete-btn" title={`Удалить маршрут «${route.name}»`} aria-label={`Удалить маршрут «${route.name}»`} onClick={() => onDelete(route)}><Trash2 size={15} /></button>
            </div>
          ))}
        </div>
        <button className="btn primary" onClick={onCreate}><Plus size={15} /> Создать маршрут</button>
      </div>
    </section>
  )
}

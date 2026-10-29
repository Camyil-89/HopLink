import { ArrowLeft, Check, Map, PanelLeftClose, PanelLeftOpen, Settings2, Trash2, UserRound, type LucideIcon } from 'lucide-react'
import appIcon from '../assets/images/hoplink-icon.png'

export type NavigationItem = { id: string; label: string; icon: LucideIcon }

type SidebarProps = {
  collapsed: boolean
  currentPage: string
  items: NavigationItem[]
  onToggle: () => void
  onNavigate: (page: string) => void
}

export function Sidebar({ collapsed, currentPage, items, onToggle, onNavigate }: SidebarProps) {
  return (
    <aside className={`sidebar ${collapsed ? 'collapsed' : ''}`}>
      <div className="brand">
        <img className="brand-mark" src={appIcon} alt="HopLink" draggable={false} />
        <span className="brand-wordmark"><b>hoplink</b><small>DESKTOP</small></span>
        <button className="icon-btn side-collapse" onClick={onToggle} title={collapsed ? 'Развернуть меню' : 'Свернуть меню'}>
          {collapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
        </button>
      </div>
      <div className="workspace-label">УПРАВЛЕНИЕ</div>
      <nav>
        {items.map(({ id, label, icon: Icon }) => (
          <button key={id} title={label} onClick={() => onNavigate(id)} className={`nav-item ${currentPage === id ? 'active' : ''}`}>
            <Icon size={17} />
            <span>{label}</span>
          </button>
        ))}
      </nav>
    </aside>
  )
}

export function Topbar({ section, routeName }: { section: string; routeName?: string }) {
  return (
    <header className="topbar">
      <div className="breadcrumbs"><span>HopLink</span><span className="slash">/</span><b>{section}{routeName ? ` / ${routeName}` : ''}</b></div>
      <div className="top-actions"><img className="brand-mark app-mark" src={appIcon} alt="HopLink" /></div>
    </header>
  )
}

type RouteHeaderProps = {
  routeName: string
  tab: 'map' | 'settings' | 'users'
  dirty: boolean
  installedServers: number
  busy: boolean
  onBack: () => void
  onTabChange: (tab: 'map' | 'settings' | 'users') => void
  onSave: () => void
  onRemoveService: () => void
}

export function RouteWorkspaceHeader({ routeName, tab, dirty, installedServers, busy, onBack, onTabChange, onSave, onRemoveService }: RouteHeaderProps) {
  return (
    <section className="route-workspace-head">
      <div className="route-workspace-title">
        <button className="text-btn route-back" onClick={onBack}><ArrowLeft size={14} /> Все маршруты</button>
        <h1>{routeName || 'Маршрут'}</h1>
        <div className="route-workspace-subline">
          <small>{tab === 'map' ? 'Карта маршрута' : tab === 'settings' ? 'Настройки и сертификаты' : 'Пользователи этого маршрута'}</small>
          <button className="text-btn route-remove-service" onClick={onRemoveService} disabled={!installedServers || busy} title={installedServers ? `Удалить сервис этого маршрута с ${installedServers} серверов` : 'Сервис маршрута ещё не установлен'}><Trash2 size={12} /> Удалить сервис</button>
        </div>
      </div>
      <div className="route-tabs">
        {tab === 'settings' && <button className="btn primary" onClick={onSave} disabled={!dirty}><Check size={14} /> Сохранить изменения</button>}
        {dirty && <span className="dirty-badge">Не сохранено</span>}
        <button className={tab === 'map' ? 'active' : ''} onClick={() => onTabChange('map')}><Map size={15} /> Карта</button>
        <button className={tab === 'settings' ? 'active' : ''} onClick={() => onTabChange('settings')}><Settings2 size={15} /> Настройки</button>
        <button className={tab === 'users' ? 'active' : ''} onClick={() => onTabChange('users')}><UserRound size={15} /> Пользователи</button>
      </div>
    </section>
  )
}

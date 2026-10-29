import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type FormEvent } from 'react'
import { addEdge, applyEdgeChanges, applyNodeChanges, type Connection, type Edge as FlowEdge, type Node as FlowNode, type OnNodesChange, type OnEdgesChange } from '@xyflow/react'
import { Map, Server as ServerIcon, Trash2 } from 'lucide-react'
import * as AppApi from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { Sidebar, Topbar, RouteWorkspaceHeader, type NavigationItem } from '../components/AppChrome'
import { emptyState, type AppState as State, type Server, type User, type VaultStatus, type RouteServiceStatus } from '../types/app'
import { initialNodes, routeSnapshot } from '../utils/flow'
import { RoutesTab } from '../tabs/RoutesTab'
import { MapTab } from '../tabs/MapTab'
import { SettingsTab } from '../tabs/SettingsTab'
import { UsersTab } from '../tabs/UsersTab'
import { ServersTab } from '../tabs/ServersTab'
import { RemoveTab } from '../tabs/RemoveTab'
import { UnsavedChangesModal } from '../modals/UnsavedChangesModal'
import { RouteModal } from '../modals/RouteModal'
import { RemoveRouteServiceModal } from '../modals/RemoveRouteServiceModal'
import { ClearMapModal } from '../modals/ClearMapModal'
import { ServerModal, type ServerDraft } from '../modals/ServerModal'
import { UserModal, type UserDraft } from '../modals/UserModal'
import { CancelApplyModal, HostApprovalModal, ProgressModal } from '../modals/OperationModals'
import appIcon from '../assets/images/hoplink-icon.png'

const api = () => AppApi
const uid = () => crypto.randomUUID()
const generatePassword = () => {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let password = ''
  while (password.length < 32) {
    const bytes = new Uint8Array(48)
    crypto.getRandomValues(bytes)
    for (const byte of bytes) {
      if (byte < 248) password += alphabet[byte % alphabet.length]
      if (password.length === 32) break
    }
  }
  return password
}
const parseHostApproval = (message: string) => {
  const fingerprint = message.match(/SHA256:[A-Za-z0-9+/]{43}/)?.[0]
  const host = message.match(/(?:для|for)\s+([^\s()]+)/i)?.[1]
  return fingerprint && host ? { host, fp: fingerprint } : null
}

export default function App(){
 const [state,setState]=useState<State>(emptyState),[nodes,setNodes]=useState<FlowNode[]>([]),[edges,setEdges]=useState<FlowEdge[]>([]),[fitRevision,setFitRevision]=useState(0),[page,setPage]=useState('routes'),[routeTab,setRouteTab]=useState<'map'|'settings'|'users'>('map'),[collapsed,setCollapsed]=useState(false),[serverForm,setServerForm]=useState(false),[userForm,setUserForm]=useState(false),[editingServer,setEditingServer]=useState<string|null>(null),[editingUser,setEditingUser]=useState<string|null>(null),[draftServer,setDraftServer]=useState({name:'',host:'',sshPort:22,sshUser:'root',sshPassword:'',sshPrivateKey:'',sshKeyPassphrase:''}),[draftUser,setDraftUser]=useState({name:'',password:''}),[selected,setSelected]=useState<string[]>([]),[activeNode,setActiveNode]=useState<string|null>(null),[busy,setBusy]=useState(false),[logs,setLogs]=useState<string[]>([]),[progressOpen,setProgressOpen]=useState(false),[progressTitle,setProgressTitle]=useState('Ход синхронизации'),[progressCancelable,setProgressCancelable]=useState(true),[operationStatus,setOperationStatus]=useState('Ожидание запуска'),[cancelConfirm,setCancelConfirm]=useState(false),[links,setLinks]=useState<Record<string,string>>({}),[notice,setNotice]=useState(''),[search,setSearch]=useState(''),[hostApproval,setHostApproval]=useState<{host:string;fp:string}|null>(null),[resumeApply,setResumeApply]=useState(false),[showSecrets,setShowSecrets]=useState(false),[vaultStatus,setVaultStatus]=useState<VaultStatus|null>(null),[vaultPassword,setVaultPassword]=useState(''),[vaultMessage,setVaultMessage]=useState(''),[routeForm,setRouteForm]=useState(false),[routeName,setRouteName]=useState(''),[routeChosen,setRouteChosen]=useState(false),[removeRouteConfirm,setRemoveRouteConfirm]=useState(false),[unsavedOpen,setUnsavedOpen]=useState(false),[pendingNavigation,setPendingNavigation]=useState<(()=>void)|null>(null)
 const [routeStatuses,setRouteStatuses]=useState<Record<string,RouteServiceStatus>>({})
 const [checkingRoutes,setCheckingRoutes]=useState(false)
 const [clearMapConfirm,setClearMapConfirm]=useState(false)
 const [operationResult,setOperationResult]=useState<'success'|'error'|'cancelled'|null>(null)
 const logEndRef=useRef<HTMLSpanElement>(null)
 const savedRouteSnapshot=useRef('')
 const currentRouteSnapshot=()=>routeSnapshot(nodes.map(node=>({...((node.data as any).server as Server),x:node.position.x,y:node.position.y})),edges.map(edge=>({id:edge.id,source:edge.source,target:edge.target})),state)
 const routeDirty=routeChosen&&savedRouteSnapshot.current!==currentRouteSnapshot()
 const refreshLinks=async()=>{const latest=await api().GetLinks() as Record<string,string>;setLinks(latest);const current=(await api().GetState()) as State;const users=current.users.map(u=>({...u,link:latest[u.name]||''}));const saved=await api().SaveUsers(users);setState(st=>({...st,...saved}));return saved as State}
 const flowNodes=useMemo(()=>nodes.map(n=>{const incoming=edges.some(e=>e.target===n.id),outgoing=edges.some(e=>e.source===n.id);const role=!incoming?'entry':!outgoing?'exit':'relay';const serviceStatus=routeStatuses[state.activeRouteId]?.servers.find(server=>server.serverId===n.id);return {...n,data:{...n.data,role,hasIn:incoming,hasOut:outgoing,serviceStatus}}}),[nodes,edges,routeStatuses,state.activeRouteId])
 const installState=(s:State)=>{setState({...emptyState,...s});setNodes(initialNodes(s.mapServers||[]));setEdges((s.edges||[]).map(e=>({...e,type:'deletable',animated:true,style:{stroke:'#5770ee',strokeWidth:2},data:{onDelete:()=>setEdges(current=>current.filter(item=>item.id!==e.id))}})));savedRouteSnapshot.current=routeSnapshot(s.mapServers||[],s.edges||[],{sni:s.sni,port:s.port,entryDomain:s.entryDomain,chainKey:s.chainKey,certificate:s.certificate,certificateKey:s.certificateKey})}
 const loadUnlockedState=async()=>{const s=await api().GetState() as State;installState(s);setVaultStatus({locked:false,needsSetup:false,hasData:true,error:''})}
 useEffect(()=>{api().GetVaultStatus().then(async(status:VaultStatus)=>{setVaultStatus(status);if(!status.locked&&(!status.needsSetup||!status.hasData))await loadUnlockedState()}).catch((e:any)=>setVaultMessage(String(e)));const off=EventsOn('deploy-log',(line:string)=>setLogs(x=>[...x,`${new Date().toLocaleTimeString()}  ${line}`]));return()=>off?.()},[])
 useEffect(()=>{logEndRef.current?.scrollIntoView({behavior:'smooth',block:'nearest'})},[logs])
 useEffect(()=>{if(!routeDirty)return;const warn=(event:BeforeUnloadEvent)=>{event.preventDefault();event.returnValue=''};window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn)},[routeDirty])
 const onNodesChange:OnNodesChange=useCallback(changes=>setNodes(ns=>applyNodeChanges(changes,ns)),[])
 const onEdgesChange:OnEdgesChange=useCallback(changes=>setEdges(es=>applyEdgeChanges(changes,es)),[])
 const canConnect=useCallback((c:Connection|FlowEdge)=>!!c.source&&!!c.target&&c.source!==c.target&&!edges.some(e=>e.target===c.target)&&!edges.some(e=>e.source===c.source),[edges])
 const onConnect=useCallback((c:Connection)=>{if(!c.source||!c.target)return;if(!canConnect(c)){setNotice('У каждого узла может быть только одно соединение слева и одно справа.');return}const id=uid();setEdges(es=>addEdge({...c,id,type:'deletable',animated:true,style:{stroke:'#5770ee',strokeWidth:2},data:{onDelete:()=>setEdges(current=>current.filter(item=>item.id!==id))}},es))},[canConnect])
 const cardServers=useMemo(()=>state.servers.filter(s=>!nodes.some(n=>n.id===s.id)&&`${s.name} ${s.host}`.toLowerCase().includes(search.toLowerCase())),[state.servers,nodes,search])
 const updateServer=(id:string,patch:Partial<Server>)=>{setNodes(ns=>ns.map(n=>n.id===id?{...n,data:{server:{...(n.data.server as Server),...patch}}}:n))}
 const handleAppError=async(error:any)=>{const message=String(error);if(message.includes('задайте пароль шифрования настроек')){const status=await api().GetVaultStatus() as VaultStatus;setVaultStatus({...status,locked:true,needsSetup:true,hasData:true});setVaultMessage('Создайте пароль, чтобы зашифровать и сохранить настройки.');setServerForm(false);setUserForm(false);setRouteForm(false);setProgressOpen(false);return}setNotice(message)}
 const addServer=async()=>{if(!draftServer.host||!draftServer.sshUser||(!draftServer.sshPassword&&!draftServer.sshPrivateKey))return;const existing=state.servers.find(s=>s.id===editingServer);const server:Server={...draftServer,id:existing?.id||uid(),name:draftServer.name||draftServer.host,x:existing?.x||60+Math.random()*350,y:existing?.y||80+Math.random()*250,role:existing?.role};const servers=existing?state.servers.map(s=>s.id===existing.id?server:s):[...state.servers,server];try{const next=await api().SaveServers(servers);setState({...state,...next});setDraftServer({name:'',host:'',sshPort:22,sshUser:'root',sshPassword:'',sshPrivateKey:'',sshKeyPassphrase:''});setEditingServer(null);setServerForm(false)}catch(e:any){await handleAppError(e)}}
 const deleteServer=async(id:string)=>{const server=state.servers.find(s=>s.id===id);if(!server)return;setProgressTitle('Удаление сервера');setProgressCancelable(false);setOperationResult(null);setLogs([]);setProgressOpen(true);setOperationStatus(`Удаление ${server.name||server.host}`);setBusy(true);setLogs([`${new Date().toLocaleTimeString()}  Удаляю сервер ${server.name||server.host} из библиотеки и карты.`]);try{const next=await api().DeleteServer(id);setState({...state,...next});setRouteStatuses({});setSelected(ids=>ids.filter(item=>item!==id));setOperationStatus('Сервер удалён');setOperationResult('success');setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  Сервер и его HopLink удалены.`]);setNotice('Сервер удалён')}catch(e:any){const msg=String(e),approval=parseHostApproval(msg);if(approval){setHostApproval(approval);setResumeApply(false)}setOperationStatus('Удаление сервера завершилось с ошибкой');setOperationResult('error');setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  Ошибка: ${msg}`]);await handleAppError(e)}finally{setBusy(false)}}
 const openAddServer=()=>{setEditingServer(null);setDraftServer({name:'',host:'',sshPort:22,sshUser:'root',sshPassword:'',sshPrivateKey:'',sshKeyPassphrase:''});setServerForm(true)}
 const openEditServer=(server:Server)=>{setEditingServer(server.id);setDraftServer({name:server.name,host:server.host,sshPort:server.sshPort,sshUser:server.sshUser,sshPassword:server.sshPassword,sshPrivateKey:server.sshPrivateKey||'',sshKeyPassphrase:server.sshKeyPassphrase||''});setServerForm(true)}
 const addUser=async()=>{if(!draftUser.name||!draftUser.password)return;const users=editingUser?state.users.map(u=>u.id===editingUser?{...u,...draftUser}:u):[...state.users,{...draftUser,id:uid(),link:'',active:true}];try{const saved=await api().SaveUsers(users);setState(st=>({...st,...saved}));setDraftUser({name:'',password:''});setEditingUser(null);setUserForm(false);setNotice('Пользователь сохранён в настройках маршрута');setTimeout(()=>setNotice(''),2200)}catch(e:any){await handleAppError(e)}}
 const toggleUser=async(id:string,active:boolean)=>{const users=state.users.map(user=>user.id===id?{...user,active,link:active?user.link:''}:user);try{const saved=await api().SaveUsers(users);setState(st=>({...st,...saved}));setNotice(active?'Пользователь активирован. Примените маршрут, чтобы добавить его на сервер.':'Пользователь отключён. Примените маршрут, чтобы удалить его с сервера.');setTimeout(()=>setNotice(''),3500)}catch(e:any){await handleAppError(e)}}
 const deleteUser=async(id:string)=>{try{const saved=await api().SaveUsers(state.users.filter(u=>u.id!==id));setState(st=>({...st,...saved}));setNotice('Пользователь удалён');setTimeout(()=>setNotice(''),2200)}catch(e:any){await handleAppError(e)}}
 const toPersist=()=>{const sv=nodes.map(n=>{const s=n.data.server as Server;return {...s,x:n.position.x,y:n.position.y}});return {sv,ed:edges.map(e=>({id:e.id,source:e.source,target:e.target}))}}
 const persistRouteDraft=async()=>{const {sv,ed}=toPersist();const saved=await api().SaveRouteDraft(sv,ed,state.sni,state.port,state.entryDomain,state.chainKey,state.certificate,state.certificateKey) as State;setState({...state,...saved});setRouteStatuses(current=>{const next={...current};delete next[state.activeRouteId];return next});savedRouteSnapshot.current=routeSnapshot(sv,ed,state);return saved}
 const saveRoute=async()=>{try{await persistRouteDraft();setNotice('Маршрут сохранён');setTimeout(()=>setNotice(''),2200)}catch(e:any){await handleAppError(e)}}
 const withUnsavedGuard=(action:()=>void)=>{if(routeDirty){setPendingNavigation(()=>action);setUnsavedOpen(true)}else action()}
 const discardAndContinue=async()=>{const action=pendingNavigation;setPendingNavigation(null);setUnsavedOpen(false);if(routeChosen){try{const saved=await api().SelectRoute(state.activeRouteId) as State;installState(saved)}catch(e:any){await handleAppError(e);return}}action?.()}
 const saveAndContinue=async()=>{try{await persistRouteDraft();setUnsavedOpen(false);const action=pendingNavigation;setPendingNavigation(null);action?.()}catch(e:any){await handleAppError(e)}}
 const navigateTo=(target:string)=>withUnsavedGuard(()=>{if(target==='routes'){setRouteChosen(false);setPage('routes');return}if(target==='remove')setRouteChosen(true);setPage(target)})
 const apply=async()=>{setResumeApply(false);setProgressTitle('Ход синхронизации');setProgressCancelable(true);setOperationResult(null);setBusy(true);setLogs([]);setLinks({});setProgressOpen(true);setOperationStatus('Подготавливаю карту к синхронизации');try{await persistRouteDraft();setOperationStatus('Подключаюсь к серверам и синхронизирую цепочку');await api().Apply();const refreshed=await refreshLinks();const {sv,ed}=toPersist();savedRouteSnapshot.current=routeSnapshot(sv,ed,refreshed);setOperationStatus('Синхронизация завершена');setOperationResult('success');setNotice('Синхронизация завершена')}catch(e:any){const msg=String(e),approval=parseHostApproval(msg);if(approval){setHostApproval(approval);setResumeApply(true)}else setResumeApply(false);const cancelled=/synchronization cancelled|context canceled|context cancelled|синхронизация отменена/i.test(msg);setOperationStatus(cancelled?'Синхронизация отменена':'Синхронизация остановлена с ошибкой');setOperationResult(cancelled?'cancelled':'error');setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  ${cancelled?'Synchronization cancelled':`Synchronization failed: ${msg}`}`]);if(msg.includes('задайте пароль шифрования настроек'))await handleAppError(e);else setNotice(cancelled?'Синхронизация отменена':msg)}finally{setBusy(false)}}
 const requestApplyCancel=async()=>{setCancelConfirm(false);try{const requested=await api().CancelApply();if(requested){setOperationStatus('Отправлен запрос на отмену');setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  Отправлен запрос на отмену синхронизации.`])}else{setNotice('Синхронизация уже завершилась или ещё не запущена.')}}catch(e:any){await handleAppError(e)}}
 const closeProgress=()=>{if(busy){if(progressCancelable)setCancelConfirm(true);return}setProgressOpen(false)}
 const approveHost=async()=>{if(!hostApproval)return;if(!/^SHA256:[A-Za-z0-9+/]{43}$/.test(hostApproval.fp)){setNotice('Fingerprint повреждён. Повторите подключение, чтобы получить его заново.');setHostApproval(null);setResumeApply(false);return}try{const trustedHost=hostApproval.host;await api().TrustHost(trustedHost,hostApproval.fp);setHostApproval(null);if(!resumeApply){setNotice(`Fingerprint ${trustedHost} сохранён.`);return}setResumeApply(false);setBusy(true);setOperationResult(null);setOperationStatus(`Fingerprint ${trustedHost} сохранён, продолжаю синхронизацию…`);setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  Fingerprint ${trustedHost} сохранён; продолжаю синхронизацию.`]);try{await api().Apply();const refreshed=await refreshLinks();const {sv,ed}=toPersist();savedRouteSnapshot.current=routeSnapshot(sv,ed,refreshed);setOperationStatus('Синхронизация завершена');setOperationResult('success');setNotice('Синхронизация завершена')}catch(e:any){const msg=String(e),approval=parseHostApproval(msg);if(approval){setHostApproval(approval);setResumeApply(true)}else setResumeApply(false);const cancelled=/synchronization cancelled|context canceled|context cancelled|синхронизация отменена/i.test(msg);setOperationStatus(cancelled?'Синхронизация отменена':'Синхронизация остановлена с ошибкой');setOperationResult(cancelled?'cancelled':'error');setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  ${cancelled?'Synchronization cancelled':`Synchronization failed: ${msg}`}`]);if(msg.includes('задайте пароль шифрования настроек'))await handleAppError(e);else setNotice(cancelled?'Синхронизация отменена':msg)}finally{setBusy(false)}}catch(e:any){await handleAppError(e)}}
 const removeSelected=async()=>{if(!selected.length)return;setProgressTitle('Удаление HopLink');setProgressCancelable(false);setOperationResult(null);setLogs([]);setProgressOpen(true);setOperationStatus(`Подготовка удаления на ${selected.length} серверах`);setBusy(true);setLogs([`${new Date().toLocaleTimeString()}  Начинаю удаление HopLink на ${selected.length} выбранных серверах.`]);try{const next=await api().RemoveHoplink(selected);setState({...state,...next});setRouteStatuses({});setOperationStatus('Удаление завершено');setOperationResult('success');setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  Удаление завершено на всех выбранных серверах.`]);setNotice('HopLink удалён с выбранных серверов')}catch(e:any){try{const latest=await api().GetState() as State;setState(current=>({...current,...latest}));setRouteStatuses({})}catch{}const msg=String(e),approval=parseHostApproval(msg);if(approval){setHostApproval(approval);setResumeApply(false)}setOperationStatus('Удаление завершилось с ошибкой');setOperationResult('error');setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  Ошибка удаления: ${msg}`]);setNotice(msg)}finally{setBusy(false)}}
 const removeRouteService=async(routeID:string)=>{
  setRemoveRouteConfirm(false)
  setProgressTitle('Удаление сервиса маршрута')
  setProgressCancelable(false)
  setOperationResult(null)
  setLogs([`${new Date().toLocaleTimeString()}  Начинаю удаление сервиса выбранного маршрута.`])
  setProgressOpen(true)
  setOperationStatus('Подключаюсь к серверам маршрута')
  setBusy(true)
  try{
   const next=await api().RemoveRouteService(routeID) as State
   setState(current=>({...current,...next}))
   setRouteStatuses(current=>{const updated={...current};delete updated[routeID];return updated})
   setLinks({})
   setOperationStatus('Сервис маршрута удалён')
   setOperationResult('success')
   setNotice('Сервис маршрута удалён с серверов')
  }catch(error:any){
   const message=String(error)
   try{const latest=await api().GetState() as State;setState(current=>({...current,...latest}))}catch{}
   const approval=parseHostApproval(message)
   if(approval){setHostApproval(approval);setResumeApply(false)}
   setOperationStatus('Удаление завершилось с ошибкой')
   setOperationResult('error')
   setLogs(lines=>[...lines,`${new Date().toLocaleTimeString()}  Ошибка удаления: ${message}`])
   setNotice(message)
  }finally{setBusy(false)}
 }
  const submitVault=async(event:FormEvent)=>{event.preventDefault();if(!vaultPassword)return;setVaultMessage('');try{if(vaultStatus?.needsSetup)await api().SetVaultPassword(vaultPassword);else await api().UnlockVault(vaultPassword);setVaultPassword('');await loadUnlockedState()}catch(e:any){setVaultPassword('');setVaultMessage(String(e))}}
  const selectRoute=(id:string)=>withUnsavedGuard(()=>{void (async()=>{if(!id||id===state.activeRouteId){setRouteChosen(true);setRouteTab('map');setPage('routes');return}try{const next=await api().SelectRoute(id) as State;installState(next);setLinks({});setActiveNode(null);setRouteChosen(true);setRouteTab('map');setPage('routes');setNotice(`Открыт маршрут «${next.routes.find(r=>r.id===id)?.name||''}»`)}catch(e:any){await handleAppError(e)}})()})
  const createRoute=async()=>{try{const next=await api().CreateRoute(routeName) as State;installState(next);setLinks({});setRouteChosen(true);setRouteTab('map');setPage('routes');setRouteName('');setRouteForm(false);setNotice('Маршрут создан')}catch(e:any){await handleAppError(e)}}
 const checkRouteServices=async()=>{
  if(checkingRoutes)return
  setCheckingRoutes(true)
  try{
   const checked=await api().CheckRouteServices() as RouteServiceStatus[]
   setRouteStatuses(Object.fromEntries(checked.map(status=>[status.routeId,status])))
  }catch(error:any){await handleAppError(error)}
  finally{setCheckingRoutes(false)}
 }
 const onDrop=(ev:DragEvent)=>{ev.preventDefault();const id=ev.dataTransfer.getData('application/hoplink-server');const s=state.servers.find(x=>x.id===id);if(!s)return;const rect=(ev.currentTarget as HTMLElement).getBoundingClientRect();const x=Math.round((ev.clientX-rect.left-100)/20)*20,y=Math.round((ev.clientY-rect.top-30)/20)*20;setNodes(ns=>[...ns,{id:s.id,position:{x,y},data:{server:{...s,x,y}},type:'server'}])}
 const onNodeClick=(_:unknown,node:FlowNode)=>setActiveNode(node.id)
 const nav:NavigationItem[]=[{id:'routes',label:'Маршруты',icon:Map},{id:'servers',label:'Серверы',icon:ServerIcon},{id:'remove',label:'Удалить HopLink',icon:Trash2}]
  if (!vaultStatus || vaultStatus.locked || (vaultStatus.needsSetup && vaultStatus.hasData)) {
    return (
      <div className="vault-screen">
        <form className="vault-card" onSubmit={submitVault}>
          <img src={appIcon} alt="HopLink" />
          <div className="vault-eyebrow">ЛОКАЛЬНОЕ ХРАНИЛИЩЕ</div>
          <h1>{vaultStatus?.needsSetup ? 'Задайте пароль' : 'Разблокировать HopLink'}</h1>
          <p>{vaultStatus?.needsSetup
            ? (vaultStatus.hasData ? 'Старые настройки будут зашифрованы этим паролем. Он не сохраняется в файл.' : 'Создайте пароль для шифрования настроек. Он будет храниться только в памяти приложения.')
            : 'Введите пароль, которым зашифрован файл настроек.'}</p>
          {vaultStatus?.error && <div className="vault-error">{vaultStatus.error}</div>}
          <label className="field">Пароль<input autoFocus type="password" value={vaultPassword} onChange={event => setVaultPassword(event.target.value)} autoComplete="current-password" required minLength={1} /></label>
          {vaultMessage && <div className="vault-error">{vaultMessage.includes('неверный пароль') ? 'Неверный пароль. Проверьте его и попробуйте ещё раз.' : vaultMessage}</div>}
          <button className="btn primary vault-submit" type="submit" disabled={!vaultPassword}>{vaultStatus?.needsSetup ? 'Создать зашифрованное хранилище' : 'Войти'}</button>
          <small>Пароль не записывается на диск. Без него содержимое настроек восстановить нельзя.</small>
        </form>
      </div>
    )
  }

  const activeRouteName = state.routes.find(route => route.id === state.activeRouteId)?.name
  return (
    <div className={`app-shell min-h-screen antialiased ${collapsed ? 'is-collapsed' : ''}`}>
      <Sidebar
        collapsed={collapsed}
        currentPage={page}
        items={nav}
        onToggle={() => setCollapsed(value => !value)}
        onNavigate={navigateTo}
      />
      <main className={`main ${page === 'routes' && routeChosen && routeTab === 'map' ? 'route-map-main' : ''}`}>
        <Topbar section={nav.find(item => item.id === page)?.label || 'Маршруты'} routeName={page === 'routes' && routeChosen ? activeRouteName : undefined} />
        {page === 'routes' && !routeChosen && <RoutesTab routes={state.routes} statuses={routeStatuses} checking={checkingRoutes} onCheck={() => void checkRouteServices()} onSelect={selectRoute} onCreate={() => { setRouteName(''); setRouteForm(true) }} />}
        {page === 'routes' && routeChosen && <RouteWorkspaceHeader
          routeName={activeRouteName || ''}
          tab={routeTab}
          dirty={routeDirty}
          installedServers={state.deployedServers?.length ?? 0}
          busy={busy}
          onBack={() => withUnsavedGuard(() => { setRouteChosen(false); setPage('routes') })}
          onTabChange={tab => withUnsavedGuard(() => setRouteTab(tab))}
          onSave={() => void saveRoute()}
          onRemoveService={() => withUnsavedGuard(() => setRemoveRouteConfirm(true))}
        />}
        {page === 'routes' && routeChosen && routeTab === 'map' && <MapTab
          state={state}
          nodes={nodes}
          flowNodes={flowNodes}
          edges={edges}
          servers={cardServers}
          search={search}
          dirty={routeDirty}
          busy={busy}
          fitRevision={fitRevision}
          onSearch={setSearch}
          onNavigateServers={() => navigateTo('servers')}
          onAddServer={server => {
            const x = Math.round((100 + Math.random() * 300) / 20) * 20
            const y = Math.round((100 + Math.random() * 200) / 20) * 20
            setNodes(current => current.some(node => node.id === server.id) ? current : [...current, { id: server.id, position: { x, y }, data: { server }, type: 'server' }])
          }}
          onDrop={onDrop}
          onClear={() => setClearMapConfirm(true)}
          onNodeDragStop={node => {
            setNodes(current => current.map(item => item.id === node.id ? { ...item, position: { x: Math.round(node.position.x / 20) * 20, y: Math.round(node.position.y / 20) * 20 } } : item))
            setFitRevision(value => value + 1)
          }}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          canConnect={canConnect}
          onEdgeClick={edge => setEdges(current => current.map(item => ({ ...item, selected: item.id === edge.id })))}
          onNodeClick={onNodeClick}
          onPaneClick={() => setActiveNode(null)}
          onSave={() => void saveRoute()}
          onApply={() => void apply()}
        />}
        {page === 'routes' && routeChosen && routeTab === 'settings' && <SettingsTab
          state={state}
          showSecrets={showSecrets}
          onChange={patch => setState(current => ({ ...current, ...patch }))}
          onToggleSecrets={() => setShowSecrets(value => !value)}
        />}
        {page === 'routes' && routeChosen && routeTab === 'users' && <UsersTab
          state={state}
          onAdd={() => { setEditingUser(null); setDraftUser({ name: '', password: '' }); setUserForm(true) }}
          onEdit={user => { setEditingUser(user.id); setDraftUser({ name: user.name, password: user.password }); setUserForm(true) }}
          onDelete={id => void deleteUser(id)}
          onCopy={user => navigator.clipboard.writeText(user.link).then(() => setNotice(`Ссылка пользователя ${user.name} скопирована`))}
          onToggle={(id, active) => void toggleUser(id, active)}
        />}
        {page === 'servers' && <ServersTab state={state} busy={busy} onAdd={openAddServer} onEdit={openEditServer} onDelete={id => void deleteServer(id)} />}
        {page === 'remove' && routeChosen && <RemoveTab
          state={state}
          selected={selected}
          busy={busy}
          onToggle={id => setSelected(current => current.includes(id) ? current.filter(item => item !== id) : [...current, id])}
          onToggleAll={() => setSelected(selected.length === state.servers.length ? [] : state.servers.map(server => server.id))}
          onRemove={() => void removeSelected()}
        />}
      </main>

      {routeForm && <RouteModal name={routeName} onNameChange={setRouteName} onClose={() => setRouteForm(false)} onCreate={() => void createRoute()} />}
      {clearMapConfirm && <ClearMapModal serverCount={nodes.length} connectionCount={edges.length} onCancel={() => setClearMapConfirm(false)} onConfirm={() => { setNodes([]); setEdges([]); setActiveNode(null); setClearMapConfirm(false) }} />}
      {removeRouteConfirm && <RemoveRouteServiceModal routeName={activeRouteName || 'Маршрут'} servers={state.deployedServers || []} onCancel={() => setRemoveRouteConfirm(false)} onConfirm={() => void removeRouteService(state.activeRouteId)} />}
      {serverForm && <ServerModal draft={draftServer} editing={!!editingServer} onChange={setDraftServer} onClose={() => setServerForm(false)} onSave={() => void addServer()} />}
      {userForm && <UserModal draft={draftUser} editing={!!editingUser} onChange={setDraftUser} onGeneratePassword={() => setDraftUser(current => ({ ...current, password: generatePassword() }))} onClose={() => setUserForm(false)} onSave={() => void addUser()} />}
      {unsavedOpen && <UnsavedChangesModal
        onStay={() => { setUnsavedOpen(false); setPendingNavigation(null) }}
        onDiscard={() => void discardAndContinue()}
        onSave={() => void saveAndContinue()}
      />}
      {progressOpen && <ProgressModal title={progressTitle} status={operationStatus} logs={logs} busy={busy} result={operationResult} cancelable={progressCancelable} endRef={logEndRef} onClose={closeProgress} onCancel={() => setCancelConfirm(true)} />}
      {hostApproval && <HostApprovalModal host={hostApproval.host} fingerprint={hostApproval.fp} onCancel={() => { setResumeApply(false); setHostApproval(null) }} onApprove={() => void approveHost()} />}
      {cancelConfirm && <CancelApplyModal onContinue={() => setCancelConfirm(false)} onConfirm={() => void requestApplyCancel()} />}
      {notice && <div className={`toast ${notice.includes('Ошибка') || notice.includes('Error') || notice.includes('fingerprint') ? 'error' : ''}`}><span>{notice}</span><button onClick={() => setNotice('')}>×</button></div>}
    </div>
  )
}

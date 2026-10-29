import { useEffect, useState } from 'react'
import { Download, RefreshCw, Settings2 } from 'lucide-react'
import * as AppApi from '../../wailsjs/go/desktop/App'

type Update = { installed: string; latest: string; available: boolean; url: string }

export function DesktopSettingsTab({ installSource, onInstallSourceChange }: {
  installSource: 'source' | 'github'
  onInstallSourceChange: (source: 'source' | 'github') => Promise<void>
}) {
  const [update, setUpdate] = useState<Update | null>(null)
  const [installed, setInstalled] = useState('…')
  const [checking, setChecking] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')

  const check = async () => {
    setChecking(true)
    setMessage('')
    try { setUpdate(await AppApi.CheckDesktopUpdate()) }
    catch (error) { setMessage(`Не удалось проверить обновления: ${String(error)}`) }
    finally { setChecking(false) }
  }
  useEffect(() => { void AppApi.GetDesktopVersion().then(setInstalled); void check() }, [])

  const saveSource = async (source: 'source' | 'github') => {
    setSaving(true)
    setMessage('')
    try { await onInstallSourceChange(source) }
    catch (error) { setMessage(`Не удалось сохранить настройку: ${String(error)}`) }
    finally { setSaving(false) }
  }

  const download = async () => {
    setMessage('')
    try { await AppApi.OpenDesktopUpdate(); setMessage('Архив обновления открыт в браузере. Распакуйте его поверх приложения после закрытия HopLink.') }
    catch (error) { setMessage(`Не удалось открыть обновление: ${String(error)}`) }
  }

  return <section className="content-page desktop-settings">
    <div className="page-head"><div><h1><Settings2 size={21} /> Настройки приложения</h1><p>Версии и способ установки HopLink на серверы</p></div></div>
    <div className="desktop-settings-card">
      <h2>Обновление desktop</h2>
      <div className="desktop-version-grid">
        <div><span>Установленная версия</span><strong>{update?.installed || installed}</strong></div>
        <div><span>Последняя версия</span><strong>{update?.latest || (checking ? 'Проверка…' : 'Недоступна')}</strong></div>
      </div>
      <div className="desktop-settings-actions">
        <button className="btn" onClick={() => void check()} disabled={checking}><RefreshCw size={14} /> Проверить</button>
        <button className="btn primary" onClick={() => void download()} disabled={!update?.available || checking}><Download size={14} /> Обновить</button>
      </div>
      <p>Обновление скачивается архивом. Закройте приложение перед заменой файлов; сохранённые настройки останутся рядом с исполняемым файлом.</p>
    </div>
    <div className="desktop-settings-card">
      <h2>Установка HopLink на сервер</h2>
      <p>Выбранный способ используется при следующем применении маршрута.</p>
      <label className="desktop-install-option"><input type="radio" name="install-source" checked={installSource === 'source'} disabled={saving} onChange={() => void saveSource('source')} /><span><strong>Собрать из исходников</strong><small>Собирает Linux-бинарник локально. Нужны исходники проекта и Go в PATH.</small></span></label>
      <label className="desktop-install-option"><input type="radio" name="install-source" checked={installSource === 'github'} disabled={saving} onChange={() => void saveSource('github')} /><span><strong>Скачать последний релиз с GitHub</strong><small>Скачивает готовый Linux-бинарник для архитектуры сервера перед применением.</small></span></label>
    </div>
    {message && <p className="desktop-settings-message">{message}</p>}
  </section>
}

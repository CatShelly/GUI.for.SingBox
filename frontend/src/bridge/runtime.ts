import { api } from './transport'

type Callback = (...args: any[]) => void
const listeners = new Map<string, Set<Callback>>()
let source: EventSource | undefined
let ready: Promise<void> | undefined
export function connectEvents() {
  if (ready) return ready
  ready = new Promise<void>((resolve, reject) => {
    source = new EventSource('/api/events')
    const timeout = setTimeout(() => reject(new Error('Event connection timed out')), 15000)
    source.addEventListener('ready', () => {
      clearTimeout(timeout)
      resolve()
      dispatch('webui:reconnected', [])
    })
    source.onmessage = (event) => {
      const { name, data } = JSON.parse(event.data)
      dispatch(name, data || [])
    }
    source.onerror = () => {
      void api('/session')
        .then((s) => {
          if (!s.authenticated) window.dispatchEvent(new Event('webui:unauthorized'))
        })
        .catch(() => {})
    }
  })
  return ready
}
export function disconnectEvents() {
  source?.close()
  source = undefined
  ready = undefined
  listeners.clear()
}
function dispatch(name: string, args: any[]) {
  listeners.get(name)?.forEach((fn) => {
    Promise.resolve()
      .then(() => fn(...args))
      .catch(console.error)
  })
}
export function EventsOn(name: string, fn: Callback) {
  if (!listeners.has(name)) listeners.set(name, new Set())
  listeners.get(name)!.add(fn)
  return () => {
    listeners.get(name)?.delete(fn)
  }
}
export function EventsOff(...names: string[]) {
  names.forEach((name) => listeners.delete(name))
}
export function EventsOffAll() {
  listeners.clear()
}
export function EventsOnce(name: string, fn: Callback) {
  const off = EventsOn(name, (...args) => {
    off()
    fn(...args)
  })
  return off
}
export function EventsOnMultiple(name: string, fn: Callback, count: number) {
  let n = 0
  const off = EventsOn(name, (...args) => {
    fn(...args)
    if (++n >= count) off()
  })
  return off
}
export function EventsEmit(name: string, ...data: any[]) {
  void api('/events', { name, data }).catch(console.error)
}
export const ClipboardGetText = async () => {
  if (!navigator.clipboard) throw new Error('浏览器读取剪贴板需要 HTTPS，请手动粘贴')
  return navigator.clipboard.readText()
}
export const ClipboardSetText = async (text: string) => {
  if (navigator.clipboard) {
    await navigator.clipboard.writeText(text)
    return true
  }
  const input = document.createElement('textarea')
  input.value = text
  input.style.position = 'fixed'
  input.style.opacity = '0'
  document.body.appendChild(input)
  input.select()
  const ok = document.execCommand('copy')
  input.remove()
  if (!ok) throw new Error('复制失败，请使用 HTTPS 或手动复制')
  return true
}
export const BrowserOpenURL = (url: string) => {
  const target = new URL(url, location.href)
  if (['http:', 'https:'].includes(target.protocol))
    window.open(target.href, '_blank', 'noopener,noreferrer')
}
export const WindowReload = () => location.reload()
export const WindowReloadApp = WindowReload
export const WindowSetTitle = (title: string) => {
  document.title = title
}
export const WindowGetSize = async () => ({ w: innerWidth, h: innerHeight })
export const WindowGetPosition = async () => ({ x: 0, y: 0 })
export const WindowIsMaximised = async () => false
export const WindowIsMinimised = async () => false
export const WindowIsNormal = async () => true
export const WindowIsFullscreen = async () => !!document.fullscreenElement
export const WindowFullscreen = () => document.documentElement.requestFullscreen()
export const WindowUnfullscreen = () => document.exitFullscreen()
export const WindowToggleMaximise = () =>
  document.fullscreenElement ? WindowUnfullscreen() : WindowFullscreen()
// Theme and layout are owned by CSS. Desktop-only compatibility setters do nothing.
export const WindowSetSystemDefaultTheme = () => {}
export const WindowSetLightTheme = () => {}
export const WindowSetDarkTheme = () => {}
export const WindowSetAlwaysOnTop = (_: boolean) => {}
export const WindowSetSize = (_w: number, _h: number) => {}
export const WindowHide = () => {}
export const WindowShow = () => window.focus()
export const WindowMinimise = () => {}
export const WindowCenter = () => {}
export const IsNotificationAvailable = async () => 'Notification' in window
export const RequestNotificationAuthorization = async () => Notification.requestPermission()
export const SendNotification = async (options: { id?: string; title: string; body?: string }) => {
  if (Notification.permission === 'granted') new Notification(options.title, { body: options.body })
}
export const CanResolveFilePaths = () => false
export const OnFileDrop = (_callback: (...args: any[]) => void, _target?: boolean) => {}
export const OnFileDropOff = () => {}

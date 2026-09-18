export async function api<T = any>(path: string, body?: unknown): Promise<T> {
  const response = await fetch('/api' + path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'Content-Type': 'application/json', 'X-WebUI-Request': '1' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (response.status === 401 && path !== '/login') {
    window.dispatchEvent(new Event('webui:unauthorized'))
  }
  const data = await response.json()
  if (!response.ok) throw new Error(data.message || `HTTP ${response.status}`)
  return data
}
export const rpc = (name: string, args: unknown[]) => api('/rpc/' + name, args)

import { useAuthStore } from '../stores/auth'

// buildWsURL 构造到后端 WebSocket 终端的地址。
// 浏览器在 WS 握手时无法自定义 Authorization 头，故令牌经 ?token= 传递。
export function buildWebShellURL(serverId: number | string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = useAuthStore.getState().token || ''
  const params = new URLSearchParams({ id: String(serverId), token })
  return `${proto}//${window.location.host}/ws/webshell?${params.toString()}`
}

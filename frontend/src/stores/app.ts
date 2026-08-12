import { create } from 'zustand'
import { settingsApi } from '../api'

interface AppState {
  platformName: string
  sshUser: string
  sshPort: number
  // 从后端设置加载平台名 / SSH 默认值，供侧边栏品牌名与 WebShell 预填使用
  loadFromSettings: () => Promise<void>
  setPlatformName: (name: string) => void
  setSSH: (user: string, port: number) => void
}

const DEFAULT_NAME = '多云管理平台'
const DEFAULT_SSH_USER = 'root'
const DEFAULT_SSH_PORT = 22

export const useAppStore = create<AppState>((set) => ({
  platformName: DEFAULT_NAME,
  sshUser: DEFAULT_SSH_USER,
  sshPort: DEFAULT_SSH_PORT,
  loadFromSettings: async () => {
    try {
      const s = await settingsApi.get()
      set({
        platformName: s.platform_name || DEFAULT_NAME,
        sshUser: s.ssh_user || DEFAULT_SSH_USER,
        sshPort: s.ssh_port || DEFAULT_SSH_PORT,
      })
    } catch {
      /* 忽略：使用默认值 */
    }
  },
  setPlatformName: (name) => set({ platformName: name || DEFAULT_NAME }),
  setSSH: (user, port) => set({ sshUser: user || DEFAULT_SSH_USER, sshPort: port || DEFAULT_SSH_PORT }),
}))

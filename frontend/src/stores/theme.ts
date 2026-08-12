import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemeMode = 'light' | 'dark'

interface ThemeState {
  mode: ThemeMode
  setMode: (mode: ThemeMode) => void
}

// 主题持久化到 localStorage，首屏即时生效，避免闪白
export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      mode: 'light',
      setMode: (mode) => set({ mode }),
    }),
    { name: 'cloud-manager-theme' }
  )
)

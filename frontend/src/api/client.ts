import axios, { type InternalAxiosRequestConfig } from 'axios'
import { message } from 'antd'
import { useAuthStore } from '../stores/auth'

const client = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

// 请求拦截：注入 JWT
client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = useAuthStore.getState().token
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：统一处理业务错误码与 401
client.interceptors.response.use(
  (response) => {
    const body = response.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code !== 0) {
        message.error(body.message || '请求失败')
        return Promise.reject(new Error(body.message))
      }
      return body.data
    }
    return response.data
  },
  (error) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().logout()
      if (window.location.pathname !== '/login') {
        message.error('登录已过期，请重新登录')
        window.location.href = '/login'
      }
    } else {
      message.error(error.response?.data?.message || error.message || '网络错误')
    }
    return Promise.reject(error)
  }
)

export default client

import axios from 'axios'
import { ElMessage } from 'element-plus'
import type { Envelope } from './types'

const client = axios.create({
  baseURL: '/',
  timeout: 15000,
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('register_token')
  if (token) {
    config.headers['X-Register-Token'] = token
  }
  return config
})

client.interceptors.response.use(
  (resp) => {
    const body = resp.data as Envelope
    if (body.code !== 0) {
      ElMessage.error(body.message || 'Request failed')
      return Promise.reject(new Error(body.message))
    }
    return resp
  },
  (err) => {
    ElMessage.error(err.message || 'Network error')
    return Promise.reject(err)
  },
)

export default client

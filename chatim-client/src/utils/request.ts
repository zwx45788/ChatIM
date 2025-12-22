import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

const service = axios.create({
  baseURL: '/api/v1',
  timeout: 10000
})

service.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers['Authorization'] = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

service.interceptors.response.use(
  (response) => {
    const res = response.data
    
    // 处理显式的 error 字段 (例如 gin.H{"error": ...})
    if (res.error) {
      ElMessage.error(res.error)
      return Promise.reject(new Error(res.error))
    }

    // 兼容 protobuf omitempty 导致 code=0 时字段丢失的情况
    // 只有当 code 存在且不为成功值时才报错
    if (res.code !== undefined && res.code !== null && res.code !== 0 && res.code !== '0' && res.code !== 200) {
      ElMessage.error(res.message || 'Error')
      
      // Handle token expiration or invalid token
      if (res.code === 401 || res.code === 1003) { // Assuming 1003 is permission denied/invalid token
         localStorage.removeItem('token')
         router.push('/login')
      }
      return Promise.reject(new Error(res.message || 'Error'))
    }
    
    return res
  },
  (error) => {
    ElMessage.error(error.message || 'Request Error')
    return Promise.reject(error)
  }
)

export default service

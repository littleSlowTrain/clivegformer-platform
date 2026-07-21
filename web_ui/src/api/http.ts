import axios from 'axios'

export const http = axios.create({ baseURL: '/api/v1', timeout: 30_000 })
http.interceptors.request.use((config) => { const token = sessionStorage.getItem('clivegformer_token'); if (token) config.headers.Authorization = `Bearer ${token}`; return config })
http.interceptors.response.use((response) => response, (error) => Promise.reject(new Error(error.response?.data?.error || error.message || '请求失败')))


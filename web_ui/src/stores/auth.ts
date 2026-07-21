import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { http } from '../api/http'
import type { User } from '../types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(JSON.parse(sessionStorage.getItem('clivegformer_user') || 'null'))
  const authenticated = computed(() => Boolean(user.value?.token))
  const save = (value: User) => { user.value = value; sessionStorage.setItem('clivegformer_user', JSON.stringify(value)); sessionStorage.setItem('clivegformer_token', value.token) }
  const login = async (username: string, password: string) => { const { data } = await http.post<User>('/auth/login', { username, password }); save(data) }
  const register = async (username: string, email: string, password: string) => { const { data } = await http.post<User>('/auth/register', { username, email, password }); save(data) }
  const logout = () => { user.value = null; sessionStorage.removeItem('clivegformer_user'); sessionStorage.removeItem('clivegformer_token') }
  return { user, authenticated, login, register, logout }
})


import { defineStore } from 'pinia'
import { ref } from 'vue'
import { http } from '../api/http'
import type { FileInfo } from '../types'

export type FileScope = 'mine' | 'team'
export interface FileFilters { region?: string; year?: number }

export const useFilesStore = defineStore('files', () => {
  const items = ref<FileInfo[]>([])
  const total = ref(0)
  const totalSize = ref(0)
  const loading = ref(false)
  const currentScope = ref<FileScope>('mine')
  const regions = ref<string[]>([])
  const years = ref<number[]>([])

  const load = async (query = '', page = 1, pageSize = 20, scope: FileScope = 'mine', filters: FileFilters = {}) => {
    loading.value = true
    currentScope.value = scope
    try {
      const { data } = await http.get('/files', { params: { query, page, page_size: pageSize, scope, region: filters.region || undefined, year: filters.year || undefined } })
      items.value = data.items || []
      total.value = data.total || 0
      totalSize.value = data.total_size || 0
    } finally {
      loading.value = false
    }
  }

  const loadFacets = async (scope: FileScope) => {
    const { data } = await http.get('/files/facets', { params: { scope } })
    regions.value = data.regions || []
    years.value = data.years || []
  }

  const download = async (id: number) => {
    const { data } = await http.post(`/files/${id}/download-ticket`)
    const configured = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, '')
    const gateway = configured || (import.meta.env.DEV ? `${window.location.protocol}//${window.location.hostname}:8080` : window.location.origin)
    const frame = document.createElement('iframe')
    frame.hidden = true
    frame.src = new URL(data.url, `${gateway}/`).toString()
    frame.title = 'file-download'
    document.body.appendChild(frame)
    window.setTimeout(() => frame.remove(), 60_000)
  }

  return { items, total, totalSize, loading, currentScope, regions, years, load, loadFacets, download }
})

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { http } from '../api/http'
import type { AnalysisResult, FileInfo } from '../types'
import { formatDate } from '../utils/format'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent])

const route = useRoute()
const variables = ref<any[]>([])
const regions = ref<string[]>([])
const years = ref<number[]>([])
const fileOptions = ref<FileInfo[]>([])
const filesLoading = ref(false)
const region = ref('')
const year = ref<number>()
const loading = ref(false)
const result = ref<AnalysisResult>()
const form = reactive({ file_id: Number(route.query.file) || undefined, type: 'ndvi', variable: 'ndvi', red_variable: 'red', nir_variable: 'nir' })
let searchTimer: number | undefined

const loadFacets = async () => {
  const { data } = await http.get('/files/facets', { params: { scope: 'team' } })
  regions.value = data.regions || []
  years.value = data.years || []
}

const searchFiles = async (query = '') => {
  filesLoading.value = true
  try {
    const { data } = await http.get('/files', {
      params: { scope: 'team', query: query || undefined, region: region.value || undefined, year: year.value, page: 1, page_size: 100 },
    })
    fileOptions.value = data.items || []
  } finally {
    filesLoading.value = false
  }
}

const remoteSearch = (query: string) => {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => searchFiles(query), 300)
}

const fileLabel = (file: FileInfo) => {
  const metadata = file.classified
    ? `区域 ${file.region_code} · 第 ${file.block_index} 块 · ${file.data_year} 年`
    : '未分类'
  return `${file.filename} · ${metadata}`
}

const loadVariables = async () => {
  if (!form.file_id) return
  const { data } = await http.get(`/analysis/files/${form.file_id}/variables`)
  variables.value = data.variables || []
  if (form.type === 'ndvi' && variables.value.some((item) => item.name === 'ndvi')) form.variable = 'ndvi'
}

onMounted(async () => {
  await loadFacets()
  if (form.file_id) {
    const { data } = await http.get(`/files/${form.file_id}`)
    fileOptions.value = [data]
    await loadVariables()
  } else {
    await searchFiles()
  }
})

watch([region, year], async () => {
  form.file_id = undefined
  variables.value = []
  result.value = undefined
  await searchFiles()
})

const run = async () => {
  if (!form.file_id) { ElMessage.warning('请先选择科研文件'); return }
  loading.value = true
  try {
    const endpoint = form.type === 'series' ? '/analysis/time-series' : '/analysis/ndvi'
    const { data } = await http.post(endpoint, { ...form, variable: form.type === 'band_ndvi' ? '' : form.variable, max_points: 500 })
    result.value = data
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

const option = computed(() => ({
  grid: { left: 58, right: 20, top: 20, bottom: 64 },
  tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: result.value?.points.map((point) => point.label), axisLabel: { rotate: 45 }, axisLine: { lineStyle: { color: '#ccd8d4' } } },
  yAxis: { type: 'value', min: form.type.includes('ndvi') ? -0.05 : undefined, max: form.type.includes('ndvi') ? 1.05 : undefined, splitLine: { lineStyle: { color: '#edf2ef' } } },
  series: [{ type: 'line', data: result.value?.points.map((point) => point.value), showSymbol: true, symbolSize: 5, smooth: true, lineStyle: { width: 3, color: '#147d72' }, areaStyle: { color: 'rgba(20,125,114,.10)' } }],
}))

const parsedJson = computed(() => JSON.stringify(result.value?.points.map((point) => ({ time: point.label, ndvi: point.value })) || [], null, 2))

interface InterpretationItem { period: string; label?: string; type?: 'rise' | 'fall' | 'anomaly'; description: string }
interface Interpretation { headline: string; overview: string; seasonal_pattern: InterpretationItem[]; key_changes: InterpretationItem[]; quality_note: string; research_suggestion: string }
const interpretation = computed<Interpretation>(() => {
  const fallback: Interpretation = { headline: 'NDVI时序分析', overview: result.value?.summary || '', seasonal_pattern: [], key_changes: [], quality_note: '', research_suggestion: '' }
  if (!result.value?.summary) return fallback
  try {
    const parsed = JSON.parse(result.value.summary)
    return { ...fallback, ...parsed, seasonal_pattern: parsed.seasonal_pattern || [], key_changes: parsed.key_changes || [] }
  } catch { return fallback }
})
const changeLabel = (type?: string) => ({ rise: '显著上升', fall: '显著下降', anomaly: '疑似异常' }[type || ''] || '关键变化')
</script>

<template>
  <div>
    <div class="page-heading"><div><span class="eyebrow">SCIENTIFIC ANALYSIS LAB</span><h1>智能分析</h1><p>按16天采样解析NetCDF的时间—NDVI数据，并将JSON交给DeepSeek进行科研解读。</p></div></div>
    <div class="analysis-grid">
      <section class="panel form-panel">
        <h3>分析参数</h3>
        <el-form label-position="top">
          <div class="analysis-facets">
            <el-form-item label="区域"><el-select v-model="region" filterable clearable placeholder="全部区域"><el-option v-for="item in regions" :key="item" :label="item" :value="item" /></el-select></el-form-item>
            <el-form-item label="年份"><el-select v-model="year" clearable placeholder="全部年份"><el-option v-for="item in years" :key="item" :label="`${item} 年`" :value="item" /></el-select></el-form-item>
          </div>
          <el-form-item label="课题组科研文件"><el-select v-model="form.file_id" filterable remote reserve-keyword :remote-method="remoteSearch" :loading="filesLoading" placeholder="输入文件名或 SHA-256 检索" @change="loadVariables"><el-option v-for="file in fileOptions" :key="file.id" :label="fileLabel(file)" :value="file.id" /></el-select></el-form-item>
          <el-form-item label="分析类型"><el-select v-model="form.type"><el-option label="NC内置NDVI智能解读" value="ndvi" /><el-option label="红光/近红外计算NDVI" value="band_ndvi" /><el-option label="普通时间序列" value="series" /></el-select></el-form-item>
          <el-form-item v-if="form.type==='ndvi'" label="NDVI变量"><el-select v-model="form.variable" filterable allow-create><el-option v-for="variable in variables" :key="variable.name" :label="variable.name" :value="variable.name" /></el-select></el-form-item>
          <template v-else-if="form.type==='band_ndvi'">
            <el-form-item label="红光变量"><el-select v-model="form.red_variable" filterable allow-create><el-option v-for="variable in variables" :key="variable.name" :label="variable.name" :value="variable.name" /></el-select></el-form-item>
            <el-form-item label="近红外变量"><el-select v-model="form.nir_variable" filterable allow-create><el-option v-for="variable in variables" :key="variable.name" :label="variable.name" :value="variable.name" /></el-select></el-form-item>
          </template>
          <el-form-item v-else label="分析变量"><el-select v-model="form.variable" filterable><el-option v-for="variable in variables" :key="variable.name" :label="variable.name" :value="variable.name" /></el-select></el-form-item>
          <el-button type="primary" size="large" :loading="loading" @click="run">解析并智能分析</el-button>
        </el-form>
      </section>

      <section class="panel result-panel">
        <div class="panel-head"><div><span class="eyebrow">ANALYSIS RESULT</span><h3>{{ result ? 'NDVI时间序列与科研解读' : '等待分析任务' }}</h3></div><div v-if="result" class="cache-meta"><span :class="['cache-pill',{ hit: result.cache_hit }]">{{ result.cache_hit ? '缓存结果' : '新分析' }}</span><small>{{ formatDate(result.generated_at) }}</small></div></div>
        <template v-if="result">
          <div class="stat-row"><div><small>最小值</small><b>{{result.minimum.toFixed(4)}}</b></div><div><small>最大值</small><b>{{result.maximum.toFixed(4)}}</b></div><div><small>平均值</small><b>{{result.mean.toFixed(4)}}</b></div><div><small>变化率</small><b>{{(result.change_rate*100).toFixed(2)}}%</b></div></div>
          <v-chart class="chart" :option="option" autoresize />
          <div class="ai-report">
            <div class="report-intro"><span class="eyebrow">AI SCIENTIFIC BRIEF</span><h2>{{ interpretation.headline }}</h2><p>{{ interpretation.overview }}</p></div>
            <div v-if="interpretation.seasonal_pattern.length || interpretation.key_changes.length" class="report-grid">
              <article v-if="interpretation.seasonal_pattern.length" class="report-card">
                <h3>季节变化</h3>
                <div v-for="item in interpretation.seasonal_pattern" :key="item.period+item.label" class="report-item"><span>{{ item.period }}</span><div><b>{{ item.label }}</b><p>{{ item.description }}</p></div></div>
              </article>
              <article v-if="interpretation.key_changes.length" class="report-card">
                <h3>关键变化</h3>
                <div v-for="item in interpretation.key_changes" :key="item.period+item.type" :class="['report-item','change',item.type]"><span>{{ changeLabel(item.type) }}</span><div><b>{{ item.period }}</b><p>{{ item.description }}</p></div></div>
              </article>
            </div>
            <div class="report-notes">
              <div v-if="interpretation.quality_note"><small>数据提示</small><p>{{ interpretation.quality_note }}</p></div>
              <div v-if="interpretation.research_suggestion"><small>下一步建议</small><p>{{ interpretation.research_suggestion }}</p></div>
            </div>
          </div>
          <el-collapse class="json-collapse"><el-collapse-item title="查看发送给DeepSeek的时间—NDVI JSON" name="json"><pre>{{ parsedJson }}</pre></el-collapse-item></el-collapse>
        </template>
        <el-empty v-else description="选择文件和变量后运行分析" />
      </section>
    </div>
  </div>
</template>

<style scoped src="../styles/analysis.css"></style>

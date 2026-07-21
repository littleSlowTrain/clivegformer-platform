<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, UploadFilled } from '@element-plus/icons-vue'
import { useFilesStore, type FileScope } from '../stores/files'
import { useUploadsStore } from '../stores/uploads'
import { formatBytes, formatDate } from '../utils/format'

const props = defineProps<{ scope: FileScope }>()
const files = useFilesStore()
const uploads = useUploadsStore()
const query = ref('')
const region = ref('')
const year = ref<number>()
const page = ref(1)
const input = ref<HTMLInputElement>()
const isTeam = computed(() => props.scope === 'team')

const load = () => files.load(query.value, page.value, 20, props.scope, { region: region.value, year: year.value })
const search = () => { page.value = 1; load() }
const reset = () => { query.value = ''; region.value = ''; year.value = undefined; page.value = 1; load() }
onMounted(async () => { await files.loadFacets(props.scope); load() })
watch(() => props.scope, async () => { page.value = 1; query.value = ''; region.value = ''; year.value = undefined; await files.loadFacets(props.scope); load() })

const choose = () => input.value?.click()
const selected = (event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.files) {
    for (const file of Array.from(target.files)) uploads.add(file).catch((error) => ElMessage.error(error.message))
  }
  target.value = ''
}
const download = (id: number) => files.download(id).catch((error) => ElMessage.error(error.message))
</script>

<template>
  <div>
    <div class="page-heading">
      <div>
        <span class="eyebrow">{{ isTeam ? 'TEAM RESEARCH CATALOG' : 'PERSONAL RESEARCH CATALOG' }}</span>
        <h1>{{ isTeam ? '课题组数据' : '我的数据' }}</h1>
        <p>{{ isTeam ? '检索、下载并分析课题组共享的完整科研数据。' : '管理自己上传的文件、目录和科研数据。' }}</p>
      </div>
      <el-button type="primary" size="large" :icon="UploadFilled" @click="choose">上传到我的数据</el-button>
      <input ref="input" type="file" multiple hidden @change="selected" />
    </div>
    <section class="panel">
      <div class="toolbar">
        <el-input v-model="query" :prefix-icon="Search" clearable placeholder="按文件名或 SHA-256 搜索" @keyup.enter="search" @clear="search" />
        <el-select v-model="region" filterable clearable placeholder="选择区域" class="facet-select"><el-option v-for="item in files.regions" :key="item" :label="`区域 ${item}`" :value="item" /></el-select>
        <el-select v-model="year" clearable placeholder="选择年份" class="facet-select"><el-option v-for="item in files.years" :key="item" :label="`${item} 年`" :value="item" /></el-select>
        <el-button @click="search">检索</el-button>
        <el-button @click="reset">重置</el-button>
      </div>
      <el-table v-loading="files.loading" :data="files.items" empty-text="没有匹配的科研文件">
        <el-table-column label="文件" min-width="260">
          <template #default="scope">
            <div class="file-cell">
              <span class="file-badge">{{ scope.row.filename.split('.').pop()?.toUpperCase() }}</span>
              <div><b>{{ scope.row.filename }}</b><small>{{ scope.row.classified ? `区域 ${scope.row.region_code} · 第 ${scope.row.block_index} 块 · ${scope.row.data_year} 年` : '未分类科研文件' }}</small></div>
            </div>
          </template>
        </el-table-column>
        <el-table-column v-if="isTeam" label="上传者" width="150">
          <template #default="scope">
            <div class="owner-cell"><b>{{ scope.row.owner_username }}</b><small>{{ scope.row.reference_count }} 位成员引用</small></div>
          </template>
        </el-table-column>
        <el-table-column v-if="isTeam" label="归属" width="110">
          <template #default="scope"><span :class="['ownership-pill', { owned: scope.row.owned_by_me }]">{{ scope.row.owned_by_me ? '我已拥有' : '组内共享' }}</span></template>
        </el-table-column>
        <el-table-column label="大小" width="130"><template #default="scope">{{ formatBytes(scope.row.file_size) }}</template></el-table-column>
        <el-table-column label="SHA-256" min-width="170"><template #default="scope"><code>{{ scope.row.file_hash.slice(0, 16) }}…</code></template></el-table-column>
        <el-table-column label="创建时间" width="180"><template #default="scope">{{ formatDate(scope.row.created_at) }}</template></el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="scope"><el-button link type="primary" @click="download(scope.row.id)">下载</el-button><router-link :to="`/analysis?file=${scope.row.id}`">分析</router-link></template>
        </el-table-column>
      </el-table>
      <el-pagination v-model:current-page="page" layout="total, prev, pager, next" :total="files.total" :page-size="20" @current-change="load" />
    </section>
  </div>
</template>

<style scoped>
.toolbar .el-input{flex:1}.facet-select{width:180px;flex:none}.owner-cell b,.owner-cell small{display:block}.owner-cell small{margin-top:4px;color:var(--muted);font-size:11px}.ownership-pill{display:inline-block;padding:4px 9px;border-radius:20px;background:#eef2f1;color:#6d807d;font-size:12px}.ownership-pill.owned{background:#ddf4eb;color:#16785d}
</style>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useFilesStore } from '../stores/files'
import { useUploadsStore } from '../stores/uploads'
import { formatBytes, formatDate } from '../utils/format'
import { greetingForHour } from '../utils/greeting'

const files = useFilesStore()
const uploads = useUploadsStore()
const greeting = ref(greetingForHour(new Date().getHours()))
let greetingTimer: number | undefined

onMounted(() => {
  files.load('', 1, 8, 'team')
  uploads.restore()
  greetingTimer = window.setInterval(() => { greeting.value = greetingForHour(new Date().getHours()) }, 60_000)
})
onBeforeUnmount(() => { if (greetingTimer) window.clearInterval(greetingTimer) })

const active = computed(() => uploads.tasks.filter((task) => ['uploading', 'hashing', 'waiting'].includes(task.status)).length)
</script>

<template>
  <div class="dashboard">
    <section class="hero-card">
      <div><span class="eyebrow light">DISTRIBUTED DATA WORKSPACE</span><h1>{{ greeting }}，今天从哪组数据开始？</h1><p>文件、上传与分析任务汇聚在一个可信的课题组数据空间中。</p></div>
      <router-link to="/files" class="hero-action">上传科研数据 <span>→</span></router-link>
    </section>
    <section class="metric-grid">
      <article><span class="metric-icon teal">文</span><small>课题组文件</small><strong>{{ files.total.toLocaleString() }}</strong><em>按物理文件去重</em></article>
      <article><span class="metric-icon blue">量</span><small>已索引容量</small><strong>{{ formatBytes(files.totalSize) }}</strong><em>课题组完整数据</em></article>
      <article><span class="metric-icon amber">传</span><small>我的进行中任务</small><strong>{{ active }}</strong><em>支持断点续传</em></article>
      <article><span class="metric-icon purple">储</span><small>对象存储</small><strong class="online">RGW 在线</strong><em>Ceph · S3 API</em></article>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="eyebrow">RECENT TEAM DATA</span><h3>最近课题组文件</h3></div><router-link to="/team-files">查看全部</router-link></div>
      <el-table :data="files.items" empty-text="暂无课题组数据，上传第一份科研文件">
        <el-table-column prop="filename" label="文件名" min-width="260" />
        <el-table-column prop="owner_username" label="上传者" />
        <el-table-column label="大小"><template #default="scope">{{ formatBytes(scope.row.file_size) }}</template></el-table-column>
        <el-table-column label="创建时间" min-width="160"><template #default="scope">{{ formatDate(scope.row.created_at) }}</template></el-table-column>
        <el-table-column prop="status" label="状态"><template #default><span class="status-pill">可用</span></template></el-table-column>
      </el-table>
    </section>
  </div>
</template>

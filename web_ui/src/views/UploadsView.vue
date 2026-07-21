<script setup lang="ts">
import { onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useUploadsStore } from '../stores/uploads'
import { formatBytes } from '../utils/format'
import type { UploadTask } from '../types'
const uploads = useUploadsStore()
const statusText: Record<UploadTask['status'], string> = { hashing: '计算Hash', waiting: '等待同Hash文件', uploading: '上传中', paused: '已暂停', finalizing: '确认中', complete: '已完成', failed: '失败', instant: '秒传完成' }
onMounted(() => uploads.restore().catch((error) => ElMessage.error(error.message)))
const resume = (task: UploadTask, event: Event) => { const input = event.target as HTMLInputElement; const file = input.files?.[0]; if (file) uploads.resume(task, file).catch((error) => ElMessage.error(error.message)); input.value = '' }
const pause = (task: UploadTask) => uploads.pause(task).catch((error) => ElMessage.error(error.message))
const cancel = (task: UploadTask) => uploads.cancel(task).catch((error) => ElMessage.error(error.message))
</script>
<template><div><div class="page-heading"><div><span class="eyebrow">RESUMABLE TRANSFER CENTER</span><h1>上传任务</h1><p>浏览器切片、SHA-256去重与Ceph断点续传状态。</p></div></div><section class="task-list"><article v-for="task in uploads.tasks" :key="task.localId" class="task-card"><div class="task-icon">{{task.filename.split('.').pop()?.toUpperCase()}}</div><div class="task-main"><div class="task-title"><div><b>{{task.filename}}</b><small>{{formatBytes(task.fileSize)}} · {{task.hash?task.hash.slice(0,18)+'…':'正在计算Hash'}}</small></div><span :class="['task-status',task.status]">{{statusText[task.status]}}</span></div><el-progress :percentage="task.status==='hashing'?task.hashProgress:task.progress" :status="task.status==='complete'||task.status==='instant'?'success':undefined" :stroke-width="8"/><p v-if="task.error" class="error-text">{{task.error}}</p></div><div class="task-actions"><label v-if="task.status==='paused'" class="el-button"><span>选择文件续传</span><input type="file" hidden @change="resume(task,$event)"/></label><el-button v-if="task.status==='uploading'" @click="pause(task)">暂停</el-button><el-button v-if="!['complete','instant','finalizing'].includes(task.status)" text type="danger" @click="cancel(task)">取消</el-button></div></article><el-empty v-if="!uploads.tasks.length" description="还没有上传任务"/></section></div></template>

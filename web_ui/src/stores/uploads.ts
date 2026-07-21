import { defineStore } from 'pinia'
import { ref } from 'vue'
import { http } from '../api/http'
import { deleteTask, loadTasks, saveTask } from '../services/task-db'
import type { UploadInfo, UploadTask } from '../types'

const workerURL = new URL('../workers/hash.worker.ts', import.meta.url)
const COMPLETE_STATUS = new Set(['COMPLETE', '3'])
const CANCELLED_STATUS = new Set(['CANCELLED', '6'])
const FAILED_STATUS = new Set(['FAILED', '4'])

export const useUploadsStore = defineStore('uploads', () => {
  const tasks = ref<UploadTask[]>([])

  const restore = async () => {
    const stored = await loadTasks()
    const reconciled = await Promise.all(stored.map(reconcileTask))
    tasks.value = reconciled.filter((task): task is UploadTask => task !== null)
  }

  const reconcileTask = async (task: UploadTask): Promise<UploadTask | null> => {
    if (task.status === 'complete' || task.status === 'instant') {
      await deleteTask(task.localId)
      return null
    }

    // A browser reload discards File objects. A record that never reached an upload
    // session cannot be resumed safely and only creates a permanent 0% ghost task.
    if (!task.uploadId) {
      if (!task.hash) {
        await deleteTask(task.localId)
        return null
      }
      const paused = { ...task, file: undefined, status: 'paused' as const, error: '请重新选择同一文件以继续' }
      await saveTask(paused)
      return paused
    }

    try {
      const { data: upload } = await http.get<UploadInfo>(`/uploads/${task.uploadId}`)
      const status = normalizeStatus(upload.status)
      if (COMPLETE_STATUS.has(status) || CANCELLED_STATUS.has(status)) {
        await deleteTask(task.localId)
        return null
      }

      const progress = upload.chunk_count > 0
        ? Math.round(((upload.parts?.length ?? 0) / upload.chunk_count) * 100)
        : task.progress
      const next: UploadTask = {
        ...task,
        file: undefined,
        hash: upload.file_hash || task.hash,
        progress,
        status: FAILED_STATUS.has(status) ? 'failed' : 'paused',
        error: FAILED_STATUS.has(status) ? '服务端上传任务失败，请取消后重新上传' : `已恢复服务端进度 ${upload.parts?.length ?? 0}/${upload.chunk_count}，请选择原文件续传`,
      }
      await saveTask(next)
      return next
    } catch {
      const paused: UploadTask = {
        ...task,
        file: undefined,
        status: 'paused',
        error: '暂时无法确认服务端状态，请稍后刷新或选择原文件续传',
      }
      await saveTask(paused)
      return paused
    }
  }

  const add = async (file: File, folder = '/') => {
    const task: UploadTask = {
      localId: crypto.randomUUID(), file, filename: file.name, folder, fileSize: file.size,
      hash: '', hashProgress: 0, progress: 0, status: 'hashing',
    }
    tasks.value.unshift(task)
    await saveTask(task)
    await hashAndUpload(task)
  }

  const hashAndUpload = (task: UploadTask) => new Promise<void>((resolve, reject) => {
    const worker = new Worker(workerURL, { type: 'module' })
    worker.onmessage = async ({ data }) => {
      if (data.type === 'progress') task.hashProgress = data.progress
      if (data.type === 'complete') {
        worker.terminate()
        task.hash = data.hash
        task.hashProgress = 100
        try {
          await saveTask(task)
          await initiate(task)
          resolve()
        } catch (error) {
          await fail(task, error)
          reject(error)
        }
      }
    }
    worker.onerror = async (error) => {
      worker.terminate()
      await fail(task, error)
      reject(error)
    }
    worker.postMessage(task.file)
  })

  const initiate = async (task: UploadTask) => {
    const { data } = await http.post('/uploads/initiate', {
      file_hash: task.hash, filename: task.filename, folder: task.folder, file_size: task.fileSize,
    })
    if (data.decision === 'INSTANT' || data.decision === 1) {
      task.status = 'instant'
      task.progress = 100
      task.error = undefined
      await deleteTask(task.localId)
      return
    }
    if (data.decision === 'WAIT' || data.decision === 3) {
      task.status = 'waiting'
      task.error = '相同文件正在上传，完成后将自动秒传'
      await saveTask(task)
      window.setTimeout(() => {
        if (task.status === 'waiting' && task.file) initiate(task).catch((error) => void fail(task, error))
      }, 5000)
      return
    }

    const upload: UploadInfo = data.upload
    task.uploadId = upload.upload_id
    task.status = 'uploading'
    task.error = undefined
    await saveTask(task)
    if (await uploadParts(task, upload)) await complete(task)
  }

  const uploadParts = async (task: UploadTask, upload: UploadInfo): Promise<boolean> => {
    if (!task.file) throw new Error('需要重新选择本地文件')
    const completed = new Set((upload.parts || []).map((part) => part.part_number))
    const queue = Array.from({ length: upload.chunk_count }, (_, index) => index + 1).filter((part) => !completed.has(part))
    let sent = completed.size
    task.progress = upload.chunk_count > 0 ? Math.round((sent / upload.chunk_count) * 100) : 0

    const runner = async () => {
      while (queue.length && task.status === 'uploading') {
        const part = queue.shift()!
        const start = (part - 1) * upload.chunk_size
        const blob = task.file!.slice(start, Math.min(task.file!.size, start + upload.chunk_size))
        try {
          await retry(() => http.put(`/uploads/${upload.upload_id}/parts/${part}`, blob, {
            headers: { 'Content-Type': 'application/octet-stream' }, timeout: 0,
          }), 3)
        } catch (error) {
          if (task.status !== 'uploading') return
          throw error
        }
        sent++
        task.progress = Math.round((sent / upload.chunk_count) * 100)
        await saveTask(task)
      }
    }
    await Promise.all(Array.from({ length: Math.min(3, queue.length || 1) }, runner))
    return task.status === 'uploading'
  }

  const complete = async (task: UploadTask) => {
    if (!task.uploadId) throw new Error('上传会话不存在')
    task.status = 'finalizing'
    task.progress = 100
    task.error = '正在确认文件元数据'
    await saveTask(task)
    await http.post(`/uploads/${task.uploadId}/complete`)
    await waitForComplete(task.uploadId)
    task.status = 'complete'
    task.error = undefined
    await deleteTask(task.localId)
  }

  const waitForComplete = async (uploadId: number) => {
    for (let attempt = 0; attempt < 60; attempt++) {
      const { data } = await http.get<UploadInfo>(`/uploads/${uploadId}`)
      if (COMPLETE_STATUS.has(normalizeStatus(data.status))) return
      await delay(500)
    }
    throw new Error('对象已写入，等待元数据确认超时；刷新页面后会自动对账')
  }

  const pause = async (task: UploadTask) => {
    const previous = task.status
    task.status = 'paused'
    task.error = '上传已暂停'
    await saveTask(task)
    try {
      if (task.uploadId) await http.post(`/uploads/${task.uploadId}/pause`)
    } catch (error) {
      task.status = previous
      task.error = undefined
      await saveTask(task)
      throw error
    }
  }

  const resume = async (task: UploadTask, file: File) => {
    if (file.size !== task.fileSize || file.name !== task.filename) throw new Error('请选择原始文件')
    task.file = file
    task.status = 'hashing'
    task.error = undefined
    task.hashProgress = 0
    await new Promise<void>((resolve, reject) => {
      const worker = new Worker(workerURL, { type: 'module' })
      worker.onmessage = async ({ data }) => {
        if (data.type === 'progress') task.hashProgress = data.progress
        if (data.type !== 'complete') return
        worker.terminate()
        if (data.hash !== task.hash) {
          task.status = 'paused'
          task.error = '所选文件Hash不一致'
          await saveTask(task)
          reject(new Error(task.error))
          return
        }
        try {
          if (task.uploadId) await http.post(`/uploads/${task.uploadId}/resume`)
          const { data: upload } = await http.get<UploadInfo>(`/uploads/${task.uploadId}`)
          if (COMPLETE_STATUS.has(normalizeStatus(upload.status))) {
            task.status = 'complete'
            task.progress = 100
            await deleteTask(task.localId)
            resolve()
            return
          }
          task.status = 'uploading'
          await saveTask(task)
          if (await uploadParts(task, upload)) await complete(task)
          resolve()
        } catch (error) {
          await fail(task, error)
          reject(error)
        }
      }
      worker.onerror = async (error) => {
        worker.terminate()
        await fail(task, error)
        reject(error)
      }
      worker.postMessage(file)
    })
  }

  const cancel = async (task: UploadTask) => {
    if (task.uploadId) await http.delete(`/uploads/${task.uploadId}`)
    await deleteTask(task.localId)
    tasks.value = tasks.value.filter((item) => item.localId !== task.localId)
  }

  const fail = async (task: UploadTask, error: unknown) => {
    if (task.status === 'paused') return
    task.status = 'failed'
    task.error = error instanceof Error ? error.message : '上传失败'
    await saveTask(task)
  }

  return { tasks, restore, add, pause, resume, cancel }
})

const normalizeStatus = (status: UploadInfo['status']) => String(status).replace('UPLOAD_STATUS_', '').toUpperCase()
const delay = (milliseconds: number) => new Promise((resolve) => window.setTimeout(resolve, milliseconds))

async function retry(action: () => Promise<unknown>, times: number) {
  let last: unknown
  for (let attempt = 0; attempt < times; attempt++) {
    try { return await action() } catch (error) { last = error; await delay(600 * 2 ** attempt) }
  }
  throw last
}

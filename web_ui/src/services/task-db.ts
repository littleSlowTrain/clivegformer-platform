import type { UploadTask } from '../types'

const DB_NAME = 'clivegformer', STORE = 'upload_tasks'
const database = () => new Promise<IDBDatabase>((resolve, reject) => { const request = indexedDB.open(DB_NAME, 1); request.onupgradeneeded = () => request.result.createObjectStore(STORE, { keyPath: 'localId' }); request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error) })

const transactionDone = (transaction: IDBTransaction) => new Promise<void>((resolve, reject) => {
  transaction.oncomplete = () => resolve()
  transaction.onerror = () => reject(transaction.error)
  transaction.onabort = () => reject(transaction.error ?? new Error('IndexedDB transaction aborted'))
})

export async function saveTask(task: UploadTask) {
  const db = await database()
  try {
    const transaction = db.transaction(STORE, 'readwrite')
    const safe = { ...task, file: undefined }
    transaction.objectStore(STORE).put(safe)
    await transactionDone(transaction)
  } finally {
    db.close()
  }
}

export async function deleteTask(localId: string) {
  const db = await database()
  try {
    const transaction = db.transaction(STORE, 'readwrite')
    transaction.objectStore(STORE).delete(localId)
    await transactionDone(transaction)
  } finally {
    db.close()
  }
}

export async function loadTasks(): Promise<UploadTask[]> {
  const db = await database()
  try {
    return await new Promise((resolve, reject) => {
      const request = db.transaction(STORE, 'readonly').objectStore(STORE).getAll()
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
  } finally {
    db.close()
  }
}

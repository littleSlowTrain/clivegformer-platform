/// <reference lib="webworker" />
import { createSHA256 } from 'hash-wasm'

self.onmessage = async (event: MessageEvent<File>) => {
  const file = event.data; const hasher = await createSHA256(); hasher.init(); const block = 4 << 20
  for (let offset = 0; offset < file.size; offset += block) { const bytes = new Uint8Array(await file.slice(offset, Math.min(file.size, offset + block)).arrayBuffer()); hasher.update(bytes); self.postMessage({ type: 'progress', progress: Math.min(100, Math.round(((offset + bytes.length) / file.size) * 100)) }) }
  self.postMessage({ type: 'complete', hash: hasher.digest('hex') })
}


export interface User { user_id: number; username: string; email: string; role: string; token: string; expires_at: number }
export interface FileInfo { id: number; file_id: number; filename: string; folder: string; file_hash: string; file_size: number; status: string; created_at: number; owner_user_id: number; owner_username: string; reference_count: number; owned_by_me: boolean; region_code: string; block_index: number; data_year: number; classified: boolean }
export interface UploadInfo { upload_id: number; file_hash: string; filename: string; folder: string; file_size: number; chunk_size: number; chunk_count: number; mode: string; status: string; parts: UploadedPart[] }
export interface UploadedPart { part_number: number; etag: string; size: number; status: string }
export interface UploadTask { localId: string; file?: File; filename: string; folder: string; fileSize: number; hash: string; hashProgress: number; uploadId?: number; progress: number; status: 'hashing'|'waiting'|'uploading'|'paused'|'finalizing'|'complete'|'failed'|'instant'; error?: string }
export interface AnalysisPoint { label: string; value: number }
export interface AnalysisResult { points: AnalysisPoint[]; minimum: number; maximum: number; mean: number; change_rate: number; missing_count: number; summary: string; analysis_result_id: number; cache_hit: boolean; generated_at: number }

import client from './client'
import type { Envelope } from './types'
import type {
  DocRecord,
  DocListParams,
  DocListResp,
  DocCreateInput,
  DocUpdateInput,
  DocCategoryRecord,
  DocCategoryCreateInput,
  DocCategoryUpdateInput,
  DocVersionRecord,
  DocVersionCreateInput,
  DocVersionDiff,
  DocSearchParams,
  DocSearchResp,
  DocAttachment,
  DocFeedbackInput,
} from '@/pages/Documents/types'

const BASE = '/api/v1/docs'

// ==================== 文档 CRUD ====================

export async function getDocuments(params: DocListParams) {
  const resp = await client.get<Envelope<DocListResp>>(BASE, { params })
  return resp.data.data
}

export async function getDocument(slug: string) {
  const resp = await client.get<Envelope<DocRecord>>(`${BASE}/${slug}`)
  return resp.data.data
}

export async function createDocument(data: DocCreateInput) {
  const resp = await client.post<Envelope<DocRecord>>(BASE, data)
  return resp.data.data
}

export async function updateDocument(slug: string, data: DocUpdateInput) {
  const resp = await client.put<Envelope<DocRecord>>(`${BASE}/${slug}`, data)
  return resp.data.data
}

export async function deleteDocument(slug: string) {
  await client.delete<Envelope<null>>(`${BASE}/${slug}`)
}

// ==================== 分类 ====================

export async function getCategories() {
  const resp = await client.get<Envelope<DocCategoryRecord[]>>(`${BASE}/categories`)
  return resp.data.data
}

export async function createCategory(data: DocCategoryCreateInput) {
  const resp = await client.post<Envelope<DocCategoryRecord>>(`${BASE}/categories`, data)
  return resp.data.data
}

export async function updateCategory(id: number, data: DocCategoryUpdateInput) {
  const resp = await client.put<Envelope<DocCategoryRecord>>(`${BASE}/categories/${id}`, data)
  return resp.data.data
}

export async function deleteCategory(id: number) {
  await client.delete<Envelope<null>>(`${BASE}/categories/${id}`)
}

// ==================== 版本 ====================

export async function getVersions(slug: string) {
  const resp = await client.get<Envelope<DocVersionRecord[]>>(`${BASE}/${slug}/versions`)
  return resp.data.data
}

export async function getVersionContent(slug: string, version: string) {
  const resp = await client.get<Envelope<{ content: string }>>(`${BASE}/${slug}/versions/${version}`)
  return resp.data.data.content
}

export async function createVersion(slug: string, data: DocVersionCreateInput) {
  const resp = await client.post<Envelope<DocVersionRecord>>(`${BASE}/${slug}/versions`, data)
  return resp.data.data
}

export async function getVersionDiff(slug: string, from: string, to: string) {
  const resp = await client.get<Envelope<DocVersionDiff>>(`${BASE}/${slug}/diff`, {
    params: { from, to },
  })
  return resp.data.data
}

// ==================== 搜索 ====================

export async function searchDocs(params: DocSearchParams) {
  const resp = await client.get<Envelope<DocSearchResp>>(`${BASE}/search`, { params })
  return resp.data.data
}

export async function searchSuggest(query: string) {
  const resp = await client.get<Envelope<string[]>>(`${BASE}/search/suggest`, {
    params: { query },
  })
  return resp.data.data
}

// ==================== 附件 ====================

export async function uploadAttachment(slug: string, file: File) {
  const formData = new FormData()
  formData.append('file', file)
  const resp = await client.post<Envelope<DocAttachment>>(
    `${BASE}/${slug}/attachments`,
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } },
  )
  return resp.data.data
}

export async function deleteAttachment(id: number) {
  await client.delete<Envelope<null>>(`${BASE}/attachments/${id}`)
}

// ==================== 反馈 ====================

export async function submitFeedback(slug: string, data: DocFeedbackInput) {
  await client.post<Envelope<null>>(`${BASE}/${slug}/feedback`, data)
}

// ==================== PDF 导出 ====================

export async function exportPdf(slug: string, version?: string) {
  const resp = await client.get(`${BASE}/${slug}/export/pdf`, {
    params: version ? { version } : undefined,
    responseType: 'blob',
  })
  return resp.data as Blob
}

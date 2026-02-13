// ============================================================
// 文档中心 - 前端展示用类型（保留原有）
// ============================================================

export interface DocumentItem {
  id: string
  title: string
  level: number
  isActive?: boolean
  badge?: string
}

export interface DocumentCategory {
  id: string
  name: string
  icon: string
  color: string
  badge?: string
  collapsed?: boolean
  items: DocumentItem[]
}

export interface DocumentVersion {
  version: string
  date: string
  isCurrent: boolean
  isStable?: boolean
}

export interface DocumentSearchResult {
  id: string
  title: string
  content: string
  category: string
  highlight?: string
}

export interface DocumentFeedback {
  type: 'content' | 'missing' | 'other'
  description: string
  email?: string
}

export interface DocumentDiff {
  type: 'added' | 'modified' | 'removed'
  title: string
  description: string
  oldContent?: string
  newContent?: string
}

// ============================================================
// 文档中心 - API 类型
// ============================================================

// 文档
export interface DocRecord {
  id: number
  title: string
  slug: string
  content: string
  category_id: number
  category_name?: string
  language: string
  status: 'draft' | 'published' | 'archived'
  author: string
  version?: string
  view_count: number
  created_at: number
  updated_at: number
}

export interface DocListParams {
  page?: number
  page_size?: number
  category_id?: number
  status?: string
  language?: string
  search?: string
}

export interface DocListResp {
  total: number
  page: number
  page_size: number
  items: DocRecord[]
}

export interface DocCreateInput {
  title: string
  slug: string
  content: string
  category_id: number
  language: string
  status: 'draft' | 'published'
}

export interface DocUpdateInput {
  title?: string
  slug?: string
  content?: string
  category_id?: number
  language?: string
  status?: 'draft' | 'published' | 'archived'
}

// 分类
export interface DocCategoryRecord {
  id: number
  name: string
  slug: string
  icon: string
  color: string
  parent_id: number | null
  sort_order: number
  children?: DocCategoryRecord[]
}

export interface DocCategoryCreateInput {
  name: string
  slug: string
  icon: string
  color: string
  parent_id?: number | null
}

export interface DocCategoryUpdateInput {
  name?: string
  slug?: string
  icon?: string
  color?: string
  parent_id?: number | null
  sort_order?: number
}

// 版本
export interface DocVersionRecord {
  id: number
  doc_id: number
  version: string
  changelog: string
  content_snapshot: string
  created_at: number
}

export interface DocVersionCreateInput {
  version: string
  changelog: string
}

export interface DocVersionDiff {
  from_version: string
  to_version: string
  changes: DocumentDiff[]
  stats: { added: number; modified: number; removed: number }
}

// 搜索
export interface DocSearchParams {
  query: string
  category_id?: number
  language?: string
  page?: number
  page_size?: number
}

export interface DocSearchHit {
  id: number
  title: string
  slug: string
  category_name: string
  snippet: string
  version: string
}

export interface DocSearchResp {
  total: number
  hits: DocSearchHit[]
  query: string
  took_ms: number
}

// 附件
export interface DocAttachment {
  id: number
  doc_id: number
  filename: string
  url: string
  size: number
  mime_type: string
  created_at: number
}

// 反馈
export interface DocFeedbackInput {
  type: 'content' | 'missing' | 'other'
  description: string
  email?: string
}

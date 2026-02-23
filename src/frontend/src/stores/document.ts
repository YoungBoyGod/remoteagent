import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as docApi from '@/api/document'
import type {
  DocRecord,
  DocListParams,
  DocCategoryRecord,
  DocCreateInput,
  DocUpdateInput,
  DocCategoryCreateInput,
  DocCategoryUpdateInput,
  DocVersionRecord,
  DocVersionCreateInput,
  DocSearchParams,
  DocSearchResp,
  DocFeedbackInput,
} from '@/pages/Documents/types'

export const useDocumentStore = defineStore('document', () => {
  // ==================== State ====================
  const documents = ref<DocRecord[]>([])
  const totalDocs = ref(0)
  const currentDoc = ref<DocRecord | null>(null)
  const categories = ref<DocCategoryRecord[]>([])
  const versions = ref<DocVersionRecord[]>([])
  const searchResults = ref<DocSearchResp | null>(null)

  // Loading states
  const loadingDocs = ref(false)
  const loadingDoc = ref(false)
  const loadingCategories = ref(false)
  const loadingVersions = ref(false)
  const saving = ref(false)
  const searching = ref(false)

  // ==================== Getters ====================
  const categoryTree = computed(() => {
    const map = new Map<number, DocCategoryRecord & { children: DocCategoryRecord[] }>()
    const roots: DocCategoryRecord[] = []

    for (const cat of categories.value) {
      map.set(cat.id, { ...cat, children: [] })
    }
    for (const cat of categories.value) {
      const node = map.get(cat.id)!
      if (cat.parent_id && map.has(cat.parent_id)) {
        map.get(cat.parent_id)!.children.push(node)
      } else {
        roots.push(node)
      }
    }
    return roots
  })

  const categoryMap = computed(() => {
    const m = new Map<number, DocCategoryRecord>()
    for (const cat of categories.value) {
      m.set(cat.id, cat)
    }
    return m
  })

  // ==================== Actions: Documents ====================
  async function fetchDocuments(params: DocListParams = {}) {
    loadingDocs.value = true
    try {
      const resp = await docApi.getDocuments(params)
      documents.value = resp.items
      totalDocs.value = resp.total
      return resp
    } finally {
      loadingDocs.value = false
    }
  }

  async function fetchDocument(slug: string) {
    loadingDoc.value = true
    try {
      currentDoc.value = await docApi.getDocument(slug)
      return currentDoc.value
    } finally {
      loadingDoc.value = false
    }
  }

  async function createDocument(data: DocCreateInput) {
    saving.value = true
    try {
      const doc = await docApi.createDocument(data)
      documents.value.unshift(doc)
      totalDocs.value++
      return doc
    } finally {
      saving.value = false
    }
  }

  async function updateDocument(slug: string, data: DocUpdateInput) {
    saving.value = true
    try {
      const doc = await docApi.updateDocument(slug, data)
      const idx = documents.value.findIndex(d => d.slug === slug)
      if (idx > -1) documents.value[idx] = doc
      if (currentDoc.value?.slug === slug) currentDoc.value = doc
      return doc
    } finally {
      saving.value = false
    }
  }

  async function deleteDocument(slug: string) {
    await docApi.deleteDocument(slug)
    documents.value = documents.value.filter(d => d.slug !== slug)
    totalDocs.value--
    if (currentDoc.value?.slug === slug) currentDoc.value = null
  }

  // ==================== Actions: Categories ====================
  function flattenCategories(cats: DocCategoryRecord[]): DocCategoryRecord[] {
    const result: DocCategoryRecord[] = []
    for (const cat of cats) {
      result.push(cat)
      if (cat.children?.length) {
        result.push(...flattenCategories(cat.children))
      }
    }
    return result
  }

  async function fetchCategories() {
    loadingCategories.value = true
    try {
      const tree = await docApi.getCategories()
      categories.value = flattenCategories(tree ?? [])
      return categories.value
    } finally {
      loadingCategories.value = false
    }
  }

  async function createCategory(data: DocCategoryCreateInput) {
    await docApi.createCategory(data)
    await fetchCategories()
  }

  async function updateCategory(id: number, data: DocCategoryUpdateInput) {
    await docApi.updateCategory(id, data)
    await fetchCategories()
  }

  async function deleteCategory(id: number) {
    await docApi.deleteCategory(id)
    await fetchCategories()
  }

  // ==================== Actions: Versions ====================
  async function fetchVersions(slug: string) {
    loadingVersions.value = true
    try {
      versions.value = await docApi.getVersions(slug)
      return versions.value
    } finally {
      loadingVersions.value = false
    }
  }

  async function createVersion(slug: string, data: DocVersionCreateInput) {
    const ver = await docApi.createVersion(slug, data)
    versions.value.unshift(ver)
    return ver
  }

  async function getVersionDiff(slug: string, from: string, to: string) {
    return await docApi.getVersionDiff(slug, from, to)
  }

  async function getVersionContent(slug: string, version: string) {
    return await docApi.getVersionContent(slug, version)
  }

  // ==================== Actions: Search ====================
  async function search(params: DocSearchParams) {
    searching.value = true
    try {
      searchResults.value = await docApi.searchDocs(params)
      return searchResults.value
    } finally {
      searching.value = false
    }
  }

  async function searchSuggest(query: string) {
    return await docApi.searchSuggest(query)
  }

  // ==================== Actions: Misc ====================
  async function submitFeedback(slug: string, data: DocFeedbackInput) {
    await docApi.submitFeedback(slug, data)
  }

  async function uploadAttachment(slug: string, file: File) {
    return await docApi.uploadAttachment(slug, file)
  }

  async function exportPdf(slug: string, version?: string) {
    return await docApi.exportPdf(slug, version)
  }

  return {
    // State
    documents,
    totalDocs,
    currentDoc,
    categories,
    versions,
    searchResults,
    loadingDocs,
    loadingDoc,
    loadingCategories,
    loadingVersions,
    saving,
    searching,
    // Getters
    categoryTree,
    categoryMap,
    // Actions
    fetchDocuments,
    fetchDocument,
    createDocument,
    updateDocument,
    deleteDocument,
    fetchCategories,
    createCategory,
    updateCategory,
    deleteCategory,
    fetchVersions,
    createVersion,
    getVersionDiff,
    getVersionContent,
    search,
    searchSuggest,
    submitFeedback,
    uploadAttachment,
    exportPdf,
  }
})

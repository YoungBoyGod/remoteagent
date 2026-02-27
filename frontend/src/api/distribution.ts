import client from './client'
import type {
  Envelope,
  DistributionCreateReq,
  DistributionUpdateReq,
  DistributionStatusReq,
  DistributionItem,
  DistributionListResp,
  DistributionS3ListReq,
  DistributionS3ListResp,
} from './types'

export async function createDistribution(data: DistributionCreateReq) {
  const resp = await client.post<Envelope<DistributionItem>>('/api/v1/distributions', data)
  return resp.data.data
}

export async function listDistributions(params: {
  page?: number
  page_size?: number
  status?: string
  search?: string
  sort_by?: string
  sort_dir?: string
}) {
  const resp = await client.get<Envelope<DistributionListResp>>('/api/v1/distributions', { params })
  return resp.data.data
}

export async function getDistribution(id: number) {
  const resp = await client.get<Envelope<DistributionItem>>(`/api/v1/distributions/${id}`)
  return resp.data.data
}

export async function updateDistribution(id: number, data: DistributionUpdateReq) {
  const resp = await client.put<Envelope<null>>(`/api/v1/distributions/${id}`, data)
  return resp.data.data
}

export async function updateDistributionStatus(id: number, data: DistributionStatusReq) {
  const resp = await client.patch<Envelope<null>>(`/api/v1/distributions/${id}/status`, data)
  return resp.data.data
}

export async function listDistributionS3Objects(params: DistributionS3ListReq) {
  const resp = await client.get<Envelope<DistributionS3ListResp>>('/api/v1/distributions/s3-objects', { params })
  return resp.data.data
}

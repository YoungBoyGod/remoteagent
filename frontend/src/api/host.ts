import client from './client'
import type { Envelope, ManagedHost, HostCreateReq, HostUpdateReq, HostListResp } from './types'

export async function listHosts(params: { page?: number; page_size?: number; status?: string; search?: string }) {
  const resp = await client.get<Envelope<HostListResp>>('/api/v1/hosts', { params })
  return resp.data.data
}

export async function getHost(hostId: string) {
  const resp = await client.get<Envelope<ManagedHost>>(`/api/v1/hosts/${hostId}`)
  return resp.data.data
}

export async function createHost(data: HostCreateReq) {
  const resp = await client.post<Envelope<{ host_id: string }>>('/api/v1/hosts', data)
  return resp.data.data
}

export async function updateHost(hostId: string, data: HostUpdateReq) {
  const resp = await client.put<Envelope<null>>(`/api/v1/hosts/${hostId}`, data)
  return resp.data.data
}

export async function deleteHost(hostId: string) {
  const resp = await client.delete<Envelope<null>>(`/api/v1/hosts/${hostId}`)
  return resp.data.data
}

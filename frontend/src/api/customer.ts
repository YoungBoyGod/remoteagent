import client from './client'
import type {
  Envelope,
  CustomerItem,
  CustomerCreateReq,
  CustomerUpdateReq,
  CustomerListResp,
  CustomerHostAssignReq,
  CustomerHostListResp,
} from './types'

export async function listCustomers(params: {
  page?: number
  page_size?: number
  status?: string
  search?: string
}) {
  const resp = await client.get<Envelope<CustomerListResp>>('/api/v1/customers', { params })
  return resp.data.data
}

export async function getCustomer(customerId: string) {
  const resp = await client.get<Envelope<CustomerItem>>(`/api/v1/customers/${customerId}`)
  return resp.data.data
}

export async function createCustomer(data: CustomerCreateReq) {
  const resp = await client.post<Envelope<{ customer_id: string }>>('/api/v1/customers', data)
  return resp.data.data
}

export async function updateCustomer(customerId: string, data: CustomerUpdateReq) {
  const resp = await client.put<Envelope<null>>(`/api/v1/customers/${customerId}`, data)
  return resp.data.data
}

export async function deleteCustomer(customerId: string) {
  const resp = await client.delete<Envelope<null>>(`/api/v1/customers/${customerId}`)
  return resp.data.data
}

export async function listCustomerHosts(customerId: string) {
  const resp = await client.get<Envelope<CustomerHostListResp>>(
    `/api/v1/customers/${customerId}/hosts`,
  )
  return resp.data.data
}

export async function assignHost(customerId: string, data: CustomerHostAssignReq) {
  const resp = await client.post<Envelope<null>>(`/api/v1/customers/${customerId}/hosts`, data)
  return resp.data.data
}

export async function unassignHost(customerId: string, hostId: string) {
  const resp = await client.delete<Envelope<null>>(
    `/api/v1/customers/${customerId}/hosts/${hostId}`,
  )
  return resp.data.data
}

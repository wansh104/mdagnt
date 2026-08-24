const API_BASE = 'http://localhost:8080'

async function request(path, options = {}) {
  const token = localStorage.getItem('medagent_token')
  const headers = { 'Content-Type': 'application/json', ...options.headers }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers })
  const text = await res.text()
  const data = text ? JSON.parse(text) : null

  if (!res.ok) {
    throw new Error(data?.error || text || `Request failed (${res.status})`)
  }
  return data
}

export const api = {
  login: (email, password) =>
    request('/api/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  register: (org_id, email, password, role) =>
    request('/api/register', { method: 'POST', body: JSON.stringify({ org_id, email, password, role }) }),
  listCases: () => request('/api/cases'),
  createCase: (payload) =>
    request('/api/cases', { method: 'POST', body: JSON.stringify(payload) }),
  submitReview: (caseId, payload) =>
    request(`/api/cases/${caseId}/review`, { method: 'POST', body: JSON.stringify(payload) }),
}
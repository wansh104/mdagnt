import { useState, useEffect } from 'react'
import { api } from './api'

function useAuth() {
  const [token, setToken] = useState(localStorage.getItem('medagent_token'))
  const [role, setRole] = useState(localStorage.getItem('medagent_role'))

  const login = (t, r) => {
    localStorage.setItem('medagent_token', t)
    localStorage.setItem('medagent_role', r)
    setToken(t); setRole(r)
  }
  const logout = () => {
    localStorage.removeItem('medagent_token')
    localStorage.removeItem('medagent_role')
    setToken(null); setRole(null)
  }
  return { token, role, login, logout }
}

function LoginScreen({ onLogin }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      const res = await api.login(email, password)
      onLogin(res.token, res.role)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="app-shell">
      <div className="topbar"><div className="brand">med<span>agent</span></div></div>
      <div style={{ maxWidth: 380, margin: '60px auto 0' }}>
        <h1>Sign in</h1>
        <p className="subtitle">Prior authorization review platform</p>
        <form onSubmit={submit} className="card">
          {error && <div className="error-box">{error}</div>}
          <label>Email</label>
          <input value={email} onChange={e => setEmail(e.target.value)} type="email" required />
          <label>Password</label>
          <input value={password} onChange={e => setPassword(e.target.value)} type="password" required />
          <button className="btn btn-primary" disabled={loading} style={{ width: '100%', justifyContent: 'center' }}>
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  )
}

function StatusPill({ status }) {
  return <span className={`status-pill status-${status}`}>{status.replace('_', ' ')}</span>
}

function CaseList({ cases, onSelectNew, onRefresh, loading }) {
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
        <div>
          <h1>Cases</h1>
          <p className="subtitle">Prior authorization requests for your organization</p>
        </div>
        <button className="btn btn-primary" onClick={onSelectNew}>+ New case</button>
      </div>
      <div className="card" style={{ padding: '4px 22px' }}>
        {loading && <div className="empty-state">Loading…</div>}
        {!loading && cases.length === 0 && (
          <div className="empty-state">No cases yet. Create one to run it through the pipeline.</div>
        )}
        {!loading && cases.map(c => (
          <div className="case-row" key={c.id}>
            <div>
              <div className="case-title">{c.treatment_requested}</div>
              <div className="case-meta">{c.id.slice(0, 8)} · {new Date(c.created_at).toLocaleString()}</div>
            </div>
            <StatusPill status={c.status} />
          </div>
        ))}
      </div>
    </div>
  )
}

function CreateCaseForm({ onCreated, onCancel }) {
  const [patientId, setPatientId] = useState('')
  const [policyId, setPolicyId] = useState('')
  const [treatment, setTreatment] = useState('')
  const [note, setNote] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      const res = await api.createCase({
        patient_id: patientId,
        policy_id: policyId,
        treatment_requested: treatment,
        clinical_note: note,
      })
      onCreated(res)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <h1>New case</h1>
      <p className="subtitle">Submitting runs the full extraction → retrieval → drafting → confidence pipeline</p>
      <form onSubmit={submit} className="card">
        {error && <div className="error-box">{error}</div>}
        <label>Patient ID</label>
        <input value={patientId} onChange={e => setPatientId(e.target.value)} placeholder="uuid" required />
        <label>Policy ID</label>
        <input value={policyId} onChange={e => setPolicyId(e.target.value)} placeholder="uuid" required />
        <label>Treatment requested</label>
        <input value={treatment} onChange={e => setTreatment(e.target.value)} placeholder="e.g. Lumbar MRI" required />
        <label>Clinical note</label>
        <textarea value={note} onChange={e => setNote(e.target.value)} required
          placeholder="Patient reports lower back pain for 8 weeks..." />
        <div className="review-actions">
          <button className="btn btn-primary" disabled={loading}>
            {loading ? 'Running pipeline… (~30s)' : 'Submit case'}
          </button>
          <button type="button" className="btn btn-ghost" onClick={onCancel}>Cancel</button>
        </div>
      </form>
    </div>
  )
}

function ClaimRow({ claim }) {
  return (
    <div className="claim-row">
      <span className={claim.supported ? 'claim-supported' : 'claim-unsupported'}>
        {claim.supported ? '✓' : '✗'}
      </span>{' '}
      {claim.claim}
      {claim.clause_id && <span className="citation-chip">{claim.clause_id.slice(0, 8)}</span>}
    </div>
  )
}

function CaseResult({ result, onReviewed, onBack }) {
  const [reviewing, setReviewing] = useState(false)
  const [error, setError] = useState('')

  const decide = async (decision) => {
    setReviewing(true); setError('')
    try {
      await api.submitReview(result.case_id, { decision })
      onReviewed()
    } catch (err) {
      setError(err.message)
      setReviewing(false)
    }
  }

  const { decision, confidence, status } = result
  const needsReview = status === 'needs_review'

  return (
    <div>
      <h1>Case result</h1>
      <p className="subtitle">{result.case_id}</p>

      <div className={`confidence-banner ${confidence.needs_human_review ? 'confidence-low' : 'confidence-high'}`}>
        <span>{confidence.reason}</span>
        <strong>{Math.round(confidence.score * 100)}% confidence</strong>
      </div>

      <div className="card">
        <h2>AI recommendation: {decision.recommendation}</h2>
        <p style={{ marginTop: 0 }}>{decision.justification}</p>
      </div>

      <div className="card">
        <h2>Claim-level citations</h2>
        {decision.claim_citations.map((c, i) => <ClaimRow claim={c} key={i} />)}
      </div>

      {error && <div className="error-box">{error}</div>}

      {needsReview ? (
        <div className="review-actions">
          <button className="btn btn-primary" disabled={reviewing} onClick={() => decide('approved')}>Approve</button>
          <button className="btn btn-danger" disabled={reviewing} onClick={() => decide('rejected')}>Reject</button>
          <button className="btn btn-ghost" onClick={onBack}>Back to cases</button>
        </div>
      ) : (
        <div className="review-actions">
          <button className="btn btn-ghost" onClick={onBack}>Back to cases</button>
        </div>
      )}
    </div>
  )
}

export default function App() {
  const { token, role, login, logout } = useAuth()
  const [view, setView] = useState('list') // list | create | result
  const [cases, setCases] = useState([])
  const [loadingCases, setLoadingCases] = useState(false)
  const [activeResult, setActiveResult] = useState(null)

  const loadCases = async () => {
    setLoadingCases(true)
    try {
      const res = await api.listCases()
      setCases(res || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoadingCases(false)
    }
  }

  useEffect(() => { if (token) loadCases() }, [token])

  if (!token) return <LoginScreen onLogin={login} />

  return (
    <div className="app-shell">
      <div className="topbar">
        <div className="brand">med<span>agent</span></div>
        <div className="topbar-right">
          <span>{role}</span>
          <button onClick={logout}>Sign out</button>
        </div>
      </div>

      {view === 'list' && (
        <CaseList
          cases={cases}
          loading={loadingCases}
          onSelectNew={() => setView('create')}
          onRefresh={loadCases}
        />
      )}

      {view === 'create' && (
        <CreateCaseForm
          onCancel={() => setView('list')}
          onCreated={(res) => { setActiveResult(res); setView('result') }}
        />
      )}

      {view === 'result' && activeResult && (
        <CaseResult
          result={activeResult}
          onReviewed={() => { setView('list'); loadCases() }}
          onBack={() => { setView('list'); loadCases() }}
        />
      )}
    </div>
  )
}
import { useState, useEffect } from 'react'

type Job = {
  id: number;
  name: string;
  status: string;
  created_at: string;
  updated_at: string;
}

function App() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [loading, setLoading] = useState(false)

  const fetchJobs = async () => {
    try {
      const res = await fetch('/api/jobs')
      if (res.ok) {
        const data = await res.json()
        setJobs(data || [])
      }
    } catch (e) {
      console.error("Failed to fetch jobs", e)
    }
  }

  useEffect(() => {
    fetchJobs()
    const interval = setInterval(fetchJobs, 2000)
    return () => clearInterval(interval)
  }, [])

  const triggerJob = async () => {
    setLoading(true)
    try {
      await fetch('/api/jobs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: `Simulation-${Math.floor(Math.random() * 1000)}` })
      })
      await fetchJobs()
    } finally {
      setLoading(false)
    }
  }

  const getStatusClass = (status: string) => {
    switch(status) {
      case 'PENDING': return 'status-pending';
      case 'PROCESSING': return 'status-processing';
      case 'COMPLETED': return 'status-completed';
      default: return 'status-failed';
    }
  }

  return (
    <div className="container">
      <header className="glass-panel">
        <h1>ACCIO Observatory</h1>
        <button onClick={triggerJob} disabled={loading} className="btn-primary">
          {loading ? 'Triggering...' : 'Trigger New Job'}
        </button>
      </header>

      <main className="glass-panel">
        <h2>Recent Jobs</h2>
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Name</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map(job => (
                <tr key={job.id}>
                  <td>#{job.id}</td>
                  <td>{job.name}</td>
                  <td><span className={`status-badge ${getStatusClass(job.status)}`}>{job.status}</span></td>
                </tr>
              ))}
              {jobs.length === 0 && (
                <tr><td colSpan={3} className="text-center">No jobs found. Trigger one to start!</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </main>
    </div>
  )
}

export default App

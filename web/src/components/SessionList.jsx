import { useEffect, useState } from 'react'

function SessionList() {
  const [sessions, setSessions] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const fetchSessions = async () => {
    try {
      const response = await fetch('/web/api/sessions')
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }
      const data = await response.json()
      setSessions(data.sessions || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: fetchSessions should only run on mount
  useEffect(() => {
    fetchSessions()
  }, [])

  const formatDateTime = (dateString) => {
    if (!dateString) return 'N/A'
    const date = new Date(dateString)
    return date.toLocaleString()
  }

  const getStatusColor = (status) => {
    switch (status) {
      case 'active':
        return 'text-green-600 bg-green-100'
      case 'completed':
        return 'text-blue-600 bg-blue-100'
      case 'failed':
        return 'text-red-600 bg-red-100'
      default:
        return 'text-gray-600 bg-gray-100'
    }
  }

  if (loading) {
    return (
      <div className="bg-white shadow rounded-lg p-6">
        <p className="text-gray-500">Loading sessions...</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className="bg-white shadow rounded-lg p-6">
        <p className="text-red-500">Error: {error}</p>
      </div>
    )
  }

  return (
    <div>
      <h2 className="text-xl font-semibold text-gray-900 mb-4">Sessions</h2>
      <div className="space-y-4">
        {sessions.length === 0 ? (
          <div className="bg-white shadow rounded-lg p-6">
            <p className="text-gray-500">No sessions found</p>
          </div>
        ) : (
          sessions.map((session) => (
            <div
              key={session.session_id}
              className="bg-white shadow rounded-lg p-6"
            >
              <div className="flex justify-between items-start">
                <div className="flex-1">
                  <p className="text-sm font-medium text-gray-900">
                    ID: {session.session_id.substring(0, 8)}...
                  </p>
                  <p className="text-sm text-gray-500">
                    Thread: {session.thread_ts}
                  </p>
                  <p className="text-sm text-gray-500">
                    {formatDateTime(session.started_at)} -{' '}
                    {session.ended_at
                      ? formatDateTime(session.ended_at)
                      : 'Active'}
                  </p>
                </div>
                <span
                  className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusColor(session.status)}`}
                >
                  {session.status}
                </span>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

export default SessionList

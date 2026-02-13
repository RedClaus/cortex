import React, { useState, useEffect } from 'react'

interface Session {
  id: string
  name: string
  active: boolean
}

const SessionList: React.FC = () => {
  const [sessions, setSessions] = useState<Session[]>([])

  const fetchSessions = async () => {
    try {
      const res = await fetch('/api/v1/sessions')
      const data = await res.json()
      setSessions(data)
    } catch (error) {
      console.error('Failed to fetch sessions', error)
    }
  }

  useEffect(() => {
    fetchSessions()
  }, [])

  const switchSession = async (id: string) => {
    // Assume POST to switch
    try {
      await fetch('/api/v1/sessions/switch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id })
      })
      fetchSessions() // Refresh
    } catch (error) {
      console.error('Switch failed', error)
    }
  }

  return (
    <div className="p-4 bg-gray-900 text-white">
      <h2 className="text-xl mb-4">Session Manager</h2>
      <div className="space-y-2">
        {sessions.map((session) => (
          <div 
            key={session.id} 
            className={"p-2 rounded cursor-pointer " + (session.active ? 'bg-teal-600' : 'bg-gray-800')}
            onClick={() => switchSession(session.id)}
          >
            {session.name} {session.active && '(Active)'}
          </div>
        ))}
      </div>
    </div>
  )
}

export default SessionList
import React, { useState, useEffect } from 'react'

interface StatusData {
  cortexBrain: string
  ollamaModels: string[]
  inferenceLanes: number
  activeSessions: number
}

const StatusPanel: React.FC = () => {
  const [status, setStatus] = useState<StatusData | null>(null)

  const fetchStatus = async () => {
    try {
      const res = await fetch('/api/v1/status')
      const data = await res.json()
      setStatus(data)
    } catch (error) {
      console.error('Failed to fetch status', error)
    }
  }

  useEffect(() => {
    fetchStatus()
    const interval = setInterval(fetchStatus, 10000)
    return () => clearInterval(interval)
  }, [])

  return (
    <div className="p-4 bg-gray-900 text-white">
      <h2 className="text-xl mb-4">System Health Dashboard</h2>
      {status ? (
        <div>
          <div className="mb-2">CortexBrain: {status.cortexBrain}</div>
          <div className="mb-2">Ollama Models: {status.ollamaModels.join(', ')}</div>
          <div className="mb-2">Inference Lanes: {status.inferenceLanes}</div>
          <div className="mb-2">Active Sessions: {status.activeSessions}</div>
        </div>
      ) : (
        <div>Loading...</div>
      )}
    </div>
  )
}

export default StatusPanel
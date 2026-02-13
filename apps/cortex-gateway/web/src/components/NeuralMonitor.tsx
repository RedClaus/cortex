import React, { useState, useEffect, useRef } from 'react'

interface Event {
  type: string
  message: string
  timestamp: string
}

const NeuralMonitor: React.FC = () => {
  const [events, setEvents] = useState<Event[]>([])
  const ws = useRef<WebSocket | null>(null)
  const logEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    ws.current = new WebSocket('ws://localhost:8080/ws/events') // Assume events endpoint
    ws.current.onmessage = (event) => {
      const data = JSON.parse(event.data)
      setEvents(prev => [...prev, data])
    }
    return () => ws.current?.close()
  }, [])

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [events])

  const getColor = (type: string) => {
    switch (type) {
      case 'info': return 'text-blue-400'
      case 'warning': return 'text-yellow-400'
      case 'error': return 'text-red-400'
      default: return 'text-white'
    }
  }

  return (
    <div className="flex flex-col h-full bg-gray-900 text-white">
      <h2 className="p-4 text-xl">Neural Bus Event Monitor</h2>
      <div className="flex-1 overflow-y-auto p-4">
        {events.map((event, i) => (
          <div key={i} className="mb-1">
            <span className="text-gray-500">{event.timestamp}</span> 
            <span className={getColor(event.type)}>[{event.type}]</span> {event.message}
          </div>
        ))}
        <div ref={logEndRef} />
      </div>
    </div>
  )
}

export default NeuralMonitor
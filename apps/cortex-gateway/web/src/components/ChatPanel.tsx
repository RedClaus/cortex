import React, { useState, useEffect, useRef } from 'react'
import ReactMarkdown from 'react-markdown'

const ChatPanel: React.FC = () => {
  const [messages, setMessages] = useState<any[]>([])
  const [input, setInput] = useState('')

  const ws = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    ws.current = new WebSocket('ws://localhost:8080/ws')
    ws.current.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      setMessages(prev => [...prev, msg])
    }
    return () => ws.current?.close()
  }, [])

  const sendMessage = () => {
    if (ws.current && input) {
      ws.current.send(JSON.stringify({ type: 'chat', content: input }))
      setInput('')
    }
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') sendMessage()
  }

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  return (
    <div className="flex flex-col h-full bg-gray-900 text-white">
      <div className="flex-1 overflow-y-auto p-4">
        {messages.map((msg, i) => (
          <div key={i} className={"mb-2 " + (msg.role === 'user' ? 'text-right' : 'text-left')}>
            <div className={"inline-block p-2 rounded " + (msg.role === 'user' ? 'bg-blue-600' : 'bg-gray-700')}>
              <ReactMarkdown>{msg.content}</ReactMarkdown>
            </div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>
      <div className="flex p-4">
        <input 
          value={input} 
          onChange={e => setInput(e.target.value)} 
          onKeyPress={handleKeyPress}
          className="flex-1 p-2 bg-gray-800 text-white border border-gray-600 rounded-l" 
          placeholder="Type a message..."
        />
        <button onClick={sendMessage} className="px-4 bg-teal-600 hover:bg-teal-700 text-white rounded-r">Send</button>
      </div>
    </div>
  )
}

export default ChatPanel
import React, { useState } from 'react'

interface MemoryItem {
  content: string
  timestamp: string
  importance: number
}

const MemoryExplorer: React.FC = () => {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<MemoryItem[]>([])

  const search = async () => {
    if (!query) return
    try {
      const res = await fetch(`/api/v1/memory/search?q=${encodeURIComponent(query)}`)
      const data = await res.json()
      setResults(data)
    } catch (error) {
      console.error('Search failed', error)
    }
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') search()
  }

  return (
    <div className="p-4 bg-gray-900 text-white">
      <h2 className="text-xl mb-4">Memory Explorer</h2>
      <div className="flex mb-4">
        <input 
          value={query} 
          onChange={e => setQuery(e.target.value)} 
          onKeyPress={handleKeyPress}
          className="flex-1 p-2 bg-gray-800 text-white border border-gray-600 rounded-l" 
          placeholder="Search memory..."
        />
        <button onClick={search} className="px-4 bg-teal-600 hover:bg-teal-700 text-white rounded-r">Search</button>
      </div>
      <div className="space-y-2">
        {results.map((item, i) => (
          <div key={i} className="p-2 bg-gray-800 rounded">
            <div className="text-sm text-gray-400">{item.timestamp} - Importance: {item.importance}</div>
            <div>{item.content}</div>
          </div>
        ))}
      </div>
    </div>
  )
}

export default MemoryExplorer
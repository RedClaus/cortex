import React, { useState } from 'react'
import ChatPanel from './ChatPanel'
import StatusPanel from './StatusPanel'
import NeuralMonitor from './NeuralMonitor'
import MemoryExplorer from './MemoryExplorer'
import SessionList from './SessionList'

type Tab = 'chat' | 'status' | 'neural' | 'memory' | 'sessions'

const Layout: React.FC = () => {
  const [activeTab, setActiveTab] = useState<Tab>('chat')

  const renderPanel = () => {
    switch (activeTab) {
      case 'chat': return <ChatPanel />
      case 'status': return <StatusPanel />
      case 'neural': return <NeuralMonitor />
      case 'memory': return <MemoryExplorer />
      case 'sessions': return <SessionList />
      default: return <ChatPanel />
    }
  }

  return (
    <div className="flex h-screen bg-gray-900 text-white">
      <div className="w-64 bg-gray-800 p-4">
        <h1 className="text-2xl mb-6 text-teal-400">Cortex Web UI</h1>
        <nav className="space-y-2">
          <button 
            onClick={() => setActiveTab('chat')} 
            className={"block w-full text-left p-2 rounded " + (activeTab === 'chat' ? 'bg-teal-600' : 'hover:bg-gray-700')}
          >
            Chat
          </button>
          <button 
            onClick={() => setActiveTab('status')} 
            className={"block w-full text-left p-2 rounded " + (activeTab === 'status' ? 'bg-teal-600' : 'hover:bg-gray-700')}
          >
            Status
          </button>
          <button 
            onClick={() => setActiveTab('neural')} 
            className={"block w-full text-left p-2 rounded " + (activeTab === 'neural' ? 'bg-teal-600' : 'hover:bg-gray-700')}
          >
            Neural
          </button>
          <button 
            onClick={() => setActiveTab('memory')} 
            className={"block w-full text-left p-2 rounded " + (activeTab === 'memory' ? 'bg-teal-600' : 'hover:bg-gray-700')}
          >
            Memory
          </button>
          <button 
            onClick={() => setActiveTab('sessions')} 
            className={"block w-full text-left p-2 rounded " + (activeTab === 'sessions' ? 'bg-teal-600' : 'hover:bg-gray-700')}
          >
            Sessions
          </button>
        </nav>
      </div>
      <div className="flex-1">
        {renderPanel()}
      </div>
    </div>
  )
}

export default Layout
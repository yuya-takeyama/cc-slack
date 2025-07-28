import React from 'react'
import ThreadList from './components/ThreadList'

function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <div className="container mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold text-gray-900 mb-8">cc-slack Sessions</h1>
        <div className="space-y-8">
          <ThreadList />
        </div>
      </div>
    </div>
  )
}

export default App
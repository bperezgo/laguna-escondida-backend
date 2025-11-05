'use client'

import { useEffect, useState } from 'react'

interface VerificationResponse {
  status: string
}

export default function Home() {
  const [verificationStatus, setVerificationStatus] = useState<string>('')
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string>('')

  useEffect(() => {
    const fetchVerification = async () => {
      try {
        setLoading(true)
        const response = await fetch('http://localhost:8080/api/verification')
        
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`)
        }
        
        const data: VerificationResponse = await response.json()
        setVerificationStatus(data.status)
        setError('')
      } catch (err) {
        setError(err instanceof Error ? err.message : 'An error occurred')
        setVerificationStatus('')
      } finally {
        setLoading(false)
      }
    }

    fetchVerification()
  }, [])

  return (
    <main style={{ 
      display: 'flex', 
      flexDirection: 'column', 
      alignItems: 'center', 
      justifyContent: 'center', 
      minHeight: '100vh',
      padding: '2rem'
    }}>
      <h1 style={{ marginBottom: '2rem', fontSize: '2rem' }}>
        Laguna Escondida
      </h1>
      
      <div style={{ 
        padding: '2rem', 
        border: '1px solid #ccc', 
        borderRadius: '8px',
        minWidth: '300px',
        textAlign: 'center'
      }}>
        <h2 style={{ marginBottom: '1rem', fontSize: '1.5rem' }}>
          Backend Verification
        </h2>
        
        {loading && <p>Loading...</p>}
        {error && (
          <p style={{ color: 'red' }}>
            Error: {error}
          </p>
        )}
        {!loading && !error && verificationStatus && (
          <p style={{ fontSize: '1.2rem', color: 'green', fontWeight: 'bold' }}>
            Status: {verificationStatus}
          </p>
        )}
      </div>
    </main>
  )
}


'use client';

import { useState } from 'react';
import { invoicesApi } from '@/lib/api/invoices';
import type { CreateElectronicInvoiceRequest } from '@/types/invoice';
import InvoiceForm from '@/components/invoices/InvoiceForm';

export default function InvoicesPageClient() {
  const [showForm, setShowForm] = useState<boolean>(false);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>('');
  const [success, setSuccess] = useState<string>('');

  const handleCreate = () => {
    setShowForm(true);
    setError('');
    setSuccess('');
  };

  const handleFormSubmit = async (data: CreateElectronicInvoiceRequest) => {
    try {
      setFormLoading(true);
      setError('');
      setSuccess('');

      console.log('data', data);

      await invoicesApi.create(data);

      setSuccess('Invoice created successfully!');
      setShowForm(false);
      
      // Clear success message after 3 seconds
      setTimeout(() => setSuccess(''), 3000);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to create invoice';
      setError(errorMessage);
      console.error('Error creating invoice:', err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setError('');
    setSuccess('');
  };

  return (
    <div style={{ 
      minHeight: '100vh',
      backgroundColor: '#f5f5f5',
      padding: '2rem',
    }}>
      <div style={{ 
        maxWidth: '1200px', 
        margin: '0 auto',
      }}>
        {/* Header */}
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: '2rem',
          flexWrap: 'wrap',
          gap: '1rem',
        }}>
          <div>
            <h1 style={{ 
              margin: 0, 
              fontSize: '2rem', 
              fontWeight: 'bold',
              color: '#333',
            }}>
              Electronic Invoices
            </h1>
            <p style={{ 
              margin: '0.5rem 0 0 0', 
              color: '#666',
              fontSize: '1rem',
            }}>
              Create and manage electronic invoices
            </p>
          </div>
          <button
            onClick={handleCreate}
            style={{
              padding: '0.75rem 1.5rem',
              backgroundColor: '#28a745',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontSize: '1rem',
              fontWeight: '500',
              boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = '#218838';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = '#28a745';
            }}
          >
            + Create Invoice
          </button>
        </div>

        {/* Success Message */}
        {success && (
          <div style={{
            padding: '1rem',
            backgroundColor: '#d4edda',
            color: '#155724',
            border: '1px solid #c3e6cb',
            borderRadius: '4px',
            marginBottom: '1.5rem',
          }}>
            <strong>Success:</strong> {success}
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div style={{
            padding: '1rem',
            backgroundColor: '#f8d7da',
            color: '#721c24',
            border: '1px solid #f5c6cb',
            borderRadius: '4px',
            marginBottom: '1.5rem',
          }}>
            <strong>Error:</strong> {error}
          </div>
        )}

        {/* Placeholder for future invoice list */}
        <div style={{
          padding: '3rem',
          textAlign: 'center',
          backgroundColor: 'white',
          borderRadius: '8px',
          border: '1px solid #e0e0e0',
        }}>
          <p style={{ color: '#666', fontSize: '1rem', margin: 0 }}>
            Click "Create Invoice" to start creating a new electronic invoice.
          </p>
        </div>

        {/* Invoice Form Modal */}
        {showForm && (
          <InvoiceForm
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
          />
        )}
      </div>
    </div>
  );
}


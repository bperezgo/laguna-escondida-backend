'use client';

import { useState } from 'react';
import type { Table } from '@/types/order';

interface TableSelectorProps {
  selectedTable: Table | null;
  onTableSelect: (table: Table) => void;
  onCreateNewTable: (tableNumber: number) => void;
}

export default function TableSelector({
  selectedTable,
  onTableSelect,
  onCreateNewTable,
}: TableSelectorProps) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newTableNumber, setNewTableNumber] = useState('');
  const [isCreating, setIsCreating] = useState(false);

  // Mock tables for now - will be replaced with API call
  const mockTables: Table[] = [
    { ID: '1', Number: 1, Status: 'available', CreatedAt: '', UpdatedAt: '' },
    { ID: '2', Number: 2, Status: 'occupied', CreatedAt: '', UpdatedAt: '' },
    { ID: '3', Number: 3, Status: 'available', CreatedAt: '', UpdatedAt: '' },
    { ID: '4', Number: 4, Status: 'available', CreatedAt: '', UpdatedAt: '' },
    { ID: '5', Number: 5, Status: 'reserved', CreatedAt: '', UpdatedAt: '' },
  ];

  const handleCreateTable = async () => {
    const tableNum = parseInt(newTableNumber);
    if (isNaN(tableNum) || tableNum <= 0) {
      alert('Please enter a valid table number');
      return;
    }

    setIsCreating(true);
    try {
      await onCreateNewTable(tableNum);
      setNewTableNumber('');
      setShowCreateForm(false);
    } catch (error) {
      console.error('Error creating table:', error);
      alert('Failed to create table');
    } finally {
      setIsCreating(false);
    }
  };

  const getStatusColor = (status: Table['Status']) => {
    switch (status) {
      case 'available':
        return '#28a745';
      case 'occupied':
        return '#dc3545';
      case 'reserved':
        return '#ffc107';
      default:
        return '#6c757d';
    }
  };

  return (
    <div style={{
      padding: '1rem',
      backgroundColor: '#f8f9fa',
      borderRadius: '8px',
      marginBottom: '1.5rem',
    }}>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: '1rem',
        flexWrap: 'wrap',
        gap: '1rem',
      }}>
        <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 'bold' }}>
          {selectedTable ? `Table ${selectedTable.Number}` : 'Select a Table'}
        </h2>
        {!showCreateForm && (
          <button
            onClick={() => setShowCreateForm(true)}
            style={{
              padding: '0.5rem 1rem',
              backgroundColor: '#007bff',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontSize: '0.875rem',
              fontWeight: '500',
            }}
          >
            + New Table
          </button>
        )}
      </div>

      {showCreateForm && (
        <div style={{
          padding: '1rem',
          backgroundColor: 'white',
          borderRadius: '4px',
          marginBottom: '1rem',
          display: 'flex',
          gap: '0.5rem',
          flexWrap: 'wrap',
        }}>
          <input
            type="number"
            placeholder="Table number"
            value={newTableNumber}
            onChange={(e) => setNewTableNumber(e.target.value)}
            style={{
              flex: 1,
              minWidth: '150px',
              padding: '0.5rem',
              border: '1px solid #ced4da',
              borderRadius: '4px',
              fontSize: '1rem',
            }}
          />
          <button
            onClick={handleCreateTable}
            disabled={isCreating}
            style={{
              padding: '0.5rem 1rem',
              backgroundColor: isCreating ? '#6c757d' : '#28a745',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: isCreating ? 'not-allowed' : 'pointer',
              fontSize: '0.875rem',
            }}
          >
            {isCreating ? 'Creating...' : 'Create'}
          </button>
          <button
            onClick={() => {
              setShowCreateForm(false);
              setNewTableNumber('');
            }}
            style={{
              padding: '0.5rem 1rem',
              backgroundColor: '#6c757d',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontSize: '0.875rem',
            }}
          >
            Cancel
          </button>
        </div>
      )}

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(100px, 1fr))',
        gap: '0.75rem',
      }}>
        {mockTables.map((table) => (
          <button
            key={table.ID}
            onClick={() => onTableSelect(table)}
            style={{
              padding: '1rem',
              backgroundColor: selectedTable?.ID === table.ID ? '#007bff' : 'white',
              color: selectedTable?.ID === table.ID ? 'white' : '#333',
              border: `2px solid ${selectedTable?.ID === table.ID ? '#007bff' : getStatusColor(table.Status)}`,
              borderRadius: '8px',
              cursor: 'pointer',
              fontSize: '1rem',
              fontWeight: 'bold',
              transition: 'all 0.2s',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: '0.5rem',
            }}
            onMouseEnter={(e) => {
              if (selectedTable?.ID !== table.ID) {
                e.currentTarget.style.transform = 'scale(1.05)';
                e.currentTarget.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)';
              }
            }}
            onMouseLeave={(e) => {
              if (selectedTable?.ID !== table.ID) {
                e.currentTarget.style.transform = 'scale(1)';
                e.currentTarget.style.boxShadow = 'none';
              }
            }}
          >
            <span>Table {table.Number}</span>
            <span style={{
              fontSize: '0.75rem',
              padding: '0.25rem 0.5rem',
              borderRadius: '4px',
              backgroundColor: selectedTable?.ID === table.ID ? 'rgba(255,255,255,0.2)' : getStatusColor(table.Status),
              color: selectedTable?.ID === table.ID ? 'white' : 'white',
              textTransform: 'capitalize',
            }}>
              {table.Status}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}


'use client';

import { useState } from 'react';
import type { Table } from '@/types/order';
import { Badge, Button, Input } from '@/components/ui';
import type { BadgeTone } from '@/components/ui';

interface TableSelectorProps {
  selectedTable: Table | null;
  onTableSelect: (table: Table) => void;
  onCreateNewTable: (tableNumber: number) => void;
}

const STATUS_LABEL: Record<Table['Status'], string> = {
  available: 'Disponible',
  occupied: 'Ocupada',
  reserved: 'Reservada',
};

const STATUS_TONE: Record<Table['Status'], BadgeTone> = {
  available: 'success',
  occupied: 'danger',
  reserved: 'warning',
};

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
      alert('Por favor ingresa un número de mesa válido');
      return;
    }

    setIsCreating(true);
    try {
      await onCreateNewTable(tableNum);
      setNewTableNumber('');
      setShowCreateForm(false);
    } catch (error) {
      console.error('Error creating table:', error);
      alert('No se pudo crear la mesa');
    } finally {
      setIsCreating(false);
    }
  };

  const getStatusBorder = (status: Table['Status']) => {
    switch (status) {
      case 'available':
        return 'var(--color-success)';
      case 'occupied':
        return 'var(--color-danger)';
      case 'reserved':
        return 'var(--color-warning)';
      default:
        return 'var(--color-border-strong)';
    }
  };

  return (
    <div
      style={{
        padding: '1rem',
        backgroundColor: 'var(--color-bg)',
        borderRadius: 'var(--radius-lg)',
        border: '1px solid var(--color-border)',
        marginBottom: '1.5rem',
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: '1rem',
          flexWrap: 'wrap',
          gap: '1rem',
        }}
      >
        <h2
          style={{
            margin: 0,
            fontSize: '1.5rem',
            fontWeight: 700,
            color: 'var(--color-text-primary)',
          }}
        >
          {selectedTable ? `Mesa ${selectedTable.Number}` : 'Seleccionar Mesa'}
        </h2>
        {!showCreateForm && (
          <Button size="sm" onClick={() => setShowCreateForm(true)}>
            + Nueva Mesa
          </Button>
        )}
      </div>

      {showCreateForm && (
        <div
          style={{
            padding: '1rem',
            backgroundColor: 'var(--color-surface)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--color-border)',
            marginBottom: '1rem',
            display: 'flex',
            gap: '0.5rem',
            alignItems: 'flex-end',
            flexWrap: 'wrap',
          }}
        >
          <div style={{ flex: 1, minWidth: '150px' }}>
            <Input
              type="number"
              label="Número de mesa"
              placeholder="Ej. 6"
              value={newTableNumber}
              onChange={(e) => setNewTableNumber(e.target.value)}
            />
          </div>
          <Button onClick={handleCreateTable} disabled={isCreating}>
            {isCreating ? 'Creando...' : 'Crear'}
          </Button>
          <Button
            variant="secondary"
            onClick={() => {
              setShowCreateForm(false);
              setNewTableNumber('');
            }}
          >
            Cancelar
          </Button>
        </div>
      )}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(110px, 1fr))',
          gap: '0.75rem',
        }}
      >
        {mockTables.map((table) => {
          const isSelected = selectedTable?.ID === table.ID;
          return (
            <button
              key={table.ID}
              type="button"
              onClick={() => onTableSelect(table)}
              style={{
                padding: '1rem',
                backgroundColor: isSelected
                  ? 'var(--color-primary)'
                  : 'var(--color-surface)',
                color: isSelected
                  ? 'var(--color-on-primary, #ffffff)'
                  : 'var(--color-text-primary)',
                border: `2px solid ${
                  isSelected ? 'var(--color-primary)' : getStatusBorder(table.Status)
                }`,
                borderRadius: 'var(--radius-md)',
                cursor: 'pointer',
                fontSize: '1rem',
                fontWeight: 700,
                transition: 'transform var(--transition-fast), box-shadow var(--transition-fast)',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: '0.5rem',
              }}
              onMouseEnter={(e) => {
                if (!isSelected) {
                  e.currentTarget.style.transform = 'translateY(-2px)';
                  e.currentTarget.style.boxShadow = 'var(--shadow-md)';
                }
              }}
              onMouseLeave={(e) => {
                if (!isSelected) {
                  e.currentTarget.style.transform = 'translateY(0)';
                  e.currentTarget.style.boxShadow = 'none';
                }
              }}
            >
              <span>Mesa {table.Number}</span>
              {isSelected ? (
                <span
                  style={{
                    fontSize: '0.7rem',
                    fontWeight: 600,
                    padding: '0.2rem 0.5rem',
                    borderRadius: 'var(--radius-sm)',
                    backgroundColor: 'rgba(255,255,255,0.2)',
                    color: 'var(--color-on-primary, #ffffff)',
                  }}
                >
                  {STATUS_LABEL[table.Status]}
                </span>
              ) : (
                <Badge tone={STATUS_TONE[table.Status]} dot={false}>
                  {STATUS_LABEL[table.Status]}
                </Badge>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}

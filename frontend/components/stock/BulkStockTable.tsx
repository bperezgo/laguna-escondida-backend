'use client';

import { useState, useEffect, useMemo } from 'react';
import type { Product } from '@/types/product';
import type { Stock, BulkStockCreationOrUpdatingRequest } from '@/types/stock';

interface BulkStockTableProps {
  products: Product[];
  stocks: Stock[];
  onSubmit: (request: BulkStockCreationOrUpdatingRequest) => Promise<void>;
  isLoading?: boolean;
}

interface ProductStockRow {
  product_id: string;
  product_name: string;
  sku: string;
  currentAmount: number;
  newAmount: number;
}

export default function BulkStockTable({ 
  products, 
  stocks, 
  onSubmit, 
  isLoading = false 
}: BulkStockTableProps) {
  // Create a map of product_id to stock amount using useMemo
  const stockMap = useMemo(() => {
    const map = new Map<string, number>();
    stocks.forEach(stock => {
      map.set(stock.product_id, stock.amount);
    });
    return map;
  }, [stocks]);

  // Initialize rows with all products using useMemo
  const initialRows = useMemo((): ProductStockRow[] => {
    return products.map(product => ({
      product_id: product.id,
      product_name: product.name,
      sku: product.sku,
      currentAmount: stockMap.get(product.id) || 0,
      newAmount: stockMap.get(product.id) || 0,
    }));
  }, [products, stockMap]);

  const [rows, setRows] = useState<ProductStockRow[]>(initialRows);
  const [originalRows, setOriginalRows] = useState<ProductStockRow[]>(initialRows);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>('');

  // Update rows when stocks or products change
  useEffect(() => {
    setRows(initialRows.map(row => ({ ...row })));
    setOriginalRows(initialRows.map(row => ({ ...row })));
  }, [initialRows]);

  const handleAmountChange = (productId: string, value: string) => {
    const numValue = value === '' ? 0 : parseInt(value, 10);
    if (isNaN(numValue) || numValue < 0) {
      return;
    }

    setRows(prevRows =>
      prevRows.map(row =>
        row.product_id === productId
          ? { ...row, newAmount: numValue }
          : row
      )
    );
    setError('');
  };

  const handleReset = () => {
    setRows(originalRows.map(row => ({ ...row })));
    setError('');
  };

  const handleSubmit = async () => {
    try {
      setSubmitting(true);
      setError('');

      // Build bulk request with only items that have changed or have non-zero amounts
      const items = rows
        .filter(row => row.newAmount !== row.currentAmount || row.newAmount > 0)
        .map(row => ({
          product_id: row.product_id,
          amount: row.newAmount,
        }));

      if (items.length === 0) {
        setError('No changes to submit');
        setSubmitting(false);
        return;
      }

      const request: BulkStockCreationOrUpdatingRequest = { items };
      await onSubmit(request);

      // Update original rows after successful submit
      setOriginalRows(rows.map(row => ({ ...row })));
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to update stocks';
      setError(errorMessage);
      console.error('Error submitting bulk stock update:', err);
    } finally {
      setSubmitting(false);
    }
  };

  const hasChanges = rows.some(row => row.newAmount !== row.currentAmount);

  return (
    <div style={{
      backgroundColor: '#fff',
      borderRadius: '8px',
      padding: '2rem',
      boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
      marginTop: '2rem',
    }}>
      <h2 style={{
        margin: '0 0 1.5rem 0',
        fontSize: '1.5rem',
        fontWeight: 'bold',
        color: '#333',
      }}>
        Bulk Stock Update
      </h2>
      <p style={{
        margin: '0 0 1.5rem 0',
        color: '#666',
        fontSize: '0.9rem',
      }}>
        Update stock amounts for all products. Current amounts are shown for reference.
      </p>

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

      <div style={{
        overflowX: 'auto',
        marginBottom: '1.5rem',
      }}>
        <table style={{
          width: '100%',
          borderCollapse: 'collapse',
          minWidth: '600px',
        }}>
          <thead>
            <tr style={{
              backgroundColor: '#f8f9fa',
              borderBottom: '2px solid #dee2e6',
            }}>
              <th style={{
                padding: '0.75rem',
                textAlign: 'left',
                fontWeight: '600',
                color: '#333',
                fontSize: '0.875rem',
                textTransform: 'uppercase',
              }}>
                Product Name
              </th>
              <th style={{
                padding: '0.75rem',
                textAlign: 'left',
                fontWeight: '600',
                color: '#333',
                fontSize: '0.875rem',
                textTransform: 'uppercase',
              }}>
                SKU
              </th>
              <th style={{
                padding: '0.75rem',
                textAlign: 'left',
                fontWeight: '600',
                color: '#333',
                fontSize: '0.875rem',
                textTransform: 'uppercase',
              }}>
                Current Amount
              </th>
              <th style={{
                padding: '0.75rem',
                textAlign: 'left',
                fontWeight: '600',
                color: '#333',
                fontSize: '0.875rem',
                textTransform: 'uppercase',
              }}>
                New Amount
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const hasChanged = row.newAmount !== row.currentAmount;
              return (
                <tr
                  key={row.product_id}
                  style={{
                    borderBottom: '1px solid #e0e0e0',
                    backgroundColor: hasChanged ? '#fff3cd' : 'transparent',
                    transition: 'background-color 0.2s',
                  }}
                >
                  <td style={{
                    padding: '0.75rem',
                    color: '#333',
                  }}>
                    {row.product_name}
                  </td>
                  <td style={{
                    padding: '0.75rem',
                    color: '#666',
                    fontFamily: 'monospace',
                    fontSize: '0.9rem',
                  }}>
                    {row.sku}
                  </td>
                  <td style={{
                    padding: '0.75rem',
                    color: '#666',
                    fontWeight: '500',
                  }}>
                    {row.currentAmount}
                  </td>
                  <td style={{
                    padding: '0.75rem',
                  }}>
                    <input
                      type="number"
                      min="0"
                      value={row.newAmount === 0 ? '' : row.newAmount}
                      onChange={(e) => handleAmountChange(row.product_id, e.target.value)}
                      style={{
                        width: '100%',
                        maxWidth: '150px',
                        padding: '0.5rem',
                        border: `1px solid ${hasChanged ? '#ffc107' : '#ced4da'}`,
                        borderRadius: '4px',
                        fontSize: '1rem',
                        boxSizing: 'border-box',
                        backgroundColor: hasChanged ? '#fff' : '#f8f9fa',
                      }}
                      placeholder="0"
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        paddingTop: '1.5rem',
        borderTop: '1px solid #e0e0e0',
        gap: '1rem',
        flexWrap: 'wrap',
      }}>
        <div style={{
          color: '#666',
          fontSize: '0.9rem',
        }}>
          {hasChanges ? (
            <span style={{ color: '#ffc107', fontWeight: '500' }}>
              You have unsaved changes
            </span>
          ) : (
            <span>No changes made</span>
          )}
        </div>
        <div style={{
          display: 'flex',
          gap: '1rem',
        }}>
          <button
            onClick={handleReset}
            disabled={submitting || !hasChanges}
            style={{
              padding: '0.75rem 1.5rem',
              backgroundColor: '#6c757d',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: (submitting || !hasChanges) ? 'not-allowed' : 'pointer',
              fontSize: '1rem',
              fontWeight: '500',
              opacity: (submitting || !hasChanges) ? 0.6 : 1,
            }}
            onMouseEnter={(e) => {
              if (!submitting && hasChanges) {
                e.currentTarget.style.backgroundColor = '#5a6268';
              }
            }}
            onMouseLeave={(e) => {
              if (!submitting && hasChanges) {
                e.currentTarget.style.backgroundColor = '#6c757d';
              }
            }}
          >
            Reset to Defaults
          </button>
          <button
            onClick={handleSubmit}
            disabled={submitting || !hasChanges || isLoading}
            style={{
              padding: '0.75rem 1.5rem',
              backgroundColor: '#28a745',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: (submitting || !hasChanges || isLoading) ? 'not-allowed' : 'pointer',
              fontSize: '1rem',
              fontWeight: '500',
              opacity: (submitting || !hasChanges || isLoading) ? 0.6 : 1,
            }}
            onMouseEnter={(e) => {
              if (!submitting && hasChanges && !isLoading) {
                e.currentTarget.style.backgroundColor = '#218838';
              }
            }}
            onMouseLeave={(e) => {
              if (!submitting && hasChanges && !isLoading) {
                e.currentTarget.style.backgroundColor = '#28a745';
              }
            }}
          >
            {submitting ? 'Submitting...' : 'Send Bulk Request'}
          </button>
        </div>
      </div>
    </div>
  );
}


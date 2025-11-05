'use client';

import type { Product } from '@/types/product';

interface ProductCardProps {
  product: Product;
  onEdit: (product: Product) => void;
  onDelete: (id: string) => void;
}

export default function ProductCard({ product, onEdit, onDelete }: ProductCardProps) {
  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(amount);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const totalPrice = product.Price + (product.Price * product.VAT / 100);

  return (
    <div style={{
      border: '1px solid #e0e0e0',
      borderRadius: '8px',
      padding: '1.5rem',
      marginBottom: '1rem',
      backgroundColor: '#fff',
      boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
      transition: 'box-shadow 0.2s',
    }}
    onMouseEnter={(e) => {
      e.currentTarget.style.boxShadow = '0 4px 8px rgba(0,0,0,0.15)';
    }}
    onMouseLeave={(e) => {
      e.currentTarget.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)';
    }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '1rem' }}>
        <div style={{ flex: 1 }}>
          <h3 style={{ margin: '0 0 0.5rem 0', fontSize: '1.25rem', fontWeight: 'bold', color: '#333' }}>
            {product.Name}
          </h3>
          <p style={{ margin: '0 0 0.5rem 0', color: '#666', fontSize: '0.9rem' }}>
            <strong>Category:</strong> {product.Category}
          </p>
          <p style={{ margin: '0 0 0.5rem 0', color: '#666', fontSize: '0.9rem' }}>
            <strong>Version:</strong> {product.Version}
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button
            onClick={() => onEdit(product)}
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
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = '#0056b3';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = '#007bff';
            }}
          >
            Edit
          </button>
          <button
            onClick={() => onDelete(product.ID)}
            style={{
              padding: '0.5rem 1rem',
              backgroundColor: '#dc3545',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontSize: '0.875rem',
              fontWeight: '500',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = '#c82333';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = '#dc3545';
            }}
          >
            Delete
          </button>
        </div>
      </div>
      
      <div style={{ 
        display: 'grid', 
        gridTemplateColumns: 'repeat(2, 1fr)', 
        gap: '1rem',
        padding: '1rem',
        backgroundColor: '#f8f9fa',
        borderRadius: '4px',
      }}>
        <div>
          <p style={{ margin: '0 0 0.25rem 0', color: '#666', fontSize: '0.85rem' }}>Base Price</p>
          <p style={{ margin: 0, fontSize: '1.1rem', fontWeight: 'bold', color: '#333' }}>
            {formatCurrency(product.Price)}
          </p>
        </div>
        <div>
          <p style={{ margin: '0 0 0.25rem 0', color: '#666', fontSize: '0.85rem' }}>VAT ({product.VAT}%)</p>
          <p style={{ margin: 0, fontSize: '1.1rem', fontWeight: 'bold', color: '#333' }}>
            {formatCurrency(product.Price * product.VAT / 100)}
          </p>
        </div>
        <div style={{ gridColumn: '1 / -1', borderTop: '1px solid #dee2e6', paddingTop: '0.5rem' }}>
          <p style={{ margin: '0 0 0.25rem 0', color: '#666', fontSize: '0.85rem' }}>Total Price</p>
          <p style={{ margin: 0, fontSize: '1.25rem', fontWeight: 'bold', color: '#28a745' }}>
            {formatCurrency(totalPrice)}
          </p>
        </div>
      </div>
      
      <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid #e0e0e0' }}>
        <p style={{ margin: '0.25rem 0', color: '#999', fontSize: '0.75rem' }}>
          Created: {formatDate(product.CreatedAt)}
        </p>
        <p style={{ margin: '0.25rem 0', color: '#999', fontSize: '0.75rem' }}>
          Updated: {formatDate(product.UpdatedAt)}
        </p>
      </div>
    </div>
  );
}


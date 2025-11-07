'use client';

interface ProductSearchProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  placeholder?: string;
}

export default function ProductSearch({
  searchQuery,
  onSearchChange,
  placeholder = 'Search products...',
}: ProductSearchProps) {
  return (
    <div style={{
      position: 'sticky',
      top: 0,
      zIndex: 10,
      backgroundColor: 'white',
      padding: '1rem',
      borderBottom: '2px solid #e0e0e0',
      boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
    }}>
      <input
        type="text"
        value={searchQuery}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder={placeholder}
        style={{
          width: '100%',
          padding: '0.75rem 1rem',
          fontSize: '1rem',
          border: '2px solid #ced4da',
          borderRadius: '8px',
          outline: 'none',
          transition: 'border-color 0.2s',
        }}
        onFocus={(e) => {
          e.currentTarget.style.borderColor = '#007bff';
        }}
        onBlur={(e) => {
          e.currentTarget.style.borderColor = '#ced4da';
        }}
      />
    </div>
  );
}


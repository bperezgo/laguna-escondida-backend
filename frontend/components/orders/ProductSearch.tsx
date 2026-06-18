'use client';

import { Input } from '@/components/ui';

interface ProductSearchProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  placeholder?: string;
}

export default function ProductSearch({
  searchQuery,
  onSearchChange,
  placeholder = 'Buscar productos...',
}: ProductSearchProps) {
  return (
    <div
      style={{
        position: 'sticky',
        top: 0,
        zIndex: 10,
        background: 'var(--color-surface)',
        padding: '1rem',
        borderBottom: '1px solid var(--color-border)',
        boxShadow: 'var(--shadow-sm)',
      }}
    >
      <Input
        type="search"
        value={searchQuery}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder={placeholder}
        aria-label="Buscar productos"
      />
    </div>
  );
}

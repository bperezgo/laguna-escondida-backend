"use client";

import { ChangeEvent } from "react";

interface OpenBillSearchProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
}

export default function OpenBillSearch({
  searchQuery,
  onSearchChange,
}: OpenBillSearchProps) {
  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    onSearchChange(e.target.value);
  };

  return (
    <div
      style={{
        marginBottom: "1.5rem",
        width: "100%",
      }}
    >
      <input
        type="text"
        placeholder="Buscar por identificador temporal..."
        value={searchQuery}
        onChange={handleChange}
        style={{
          width: "100%",
          padding: "0.875rem 1rem",
          fontSize: "1rem",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-md)",
          outline: "none",
          transition: "border-color var(--transition-normal)",
          backgroundColor: "var(--color-surface)",
          color: "var(--color-text-primary)",
        }}
        onFocus={(e) => {
          e.currentTarget.style.borderColor = "var(--color-primary)";
        }}
        onBlur={(e) => {
          e.currentTarget.style.borderColor = "var(--color-border)";
        }}
      />
    </div>
  );
}

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
        placeholder="Search by temporal identifier..."
        value={searchQuery}
        onChange={handleChange}
        style={{
          width: "100%",
          padding: "0.875rem 1rem",
          fontSize: "1rem",
          border: "2px solid #e0e0e0",
          borderRadius: "8px",
          outline: "none",
          transition: "border-color 0.2s",
        }}
        onFocus={(e) => {
          e.currentTarget.style.borderColor = "#007bff";
        }}
        onBlur={(e) => {
          e.currentTarget.style.borderColor = "#e0e0e0";
        }}
      />
    </div>
  );
}

"use client";

import { ChangeEvent } from "react";
import { Input } from "@/components/ui";

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
    <div style={{ marginBottom: "1.5rem", width: "100%" }}>
      <Input
        type="search"
        placeholder="Buscar por identificador temporal..."
        value={searchQuery}
        onChange={handleChange}
        aria-label="Buscar cuentas abiertas"
      />
    </div>
  );
}

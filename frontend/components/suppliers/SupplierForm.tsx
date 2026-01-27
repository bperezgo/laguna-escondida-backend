"use client";

import { useState, useEffect } from "react";
import type {
  Supplier,
  CreateSupplierRequest,
  UpdateSupplierRequest,
} from "@/types/supplier";

interface SupplierFormProps {
  supplier?: Supplier | null;
  onSubmit: (
    data: CreateSupplierRequest | UpdateSupplierRequest
  ) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

export default function SupplierForm({
  supplier,
  onSubmit,
  onCancel,
  isLoading = false,
}: SupplierFormProps) {
  const [formData, setFormData] = useState({
    name: supplier?.name || "",
    contact_name: supplier?.contact_name || "",
    phone: supplier?.phone || "",
    email: supplier?.email || "",
    notes: supplier?.notes || "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (supplier) {
      setFormData({
        name: supplier.name || "",
        contact_name: supplier.contact_name || "",
        phone: supplier.phone || "",
        email: supplier.email || "",
        notes: supplier.notes || "",
      });
    }
  }, [supplier]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Name: required, min=1, max=255
    if (!formData.name.trim()) {
      newErrors.name = "El nombre del proveedor es requerido";
    } else if (formData.name.length > 255) {
      newErrors.name =
        "El nombre del proveedor debe tener 255 caracteres o menos";
    }

    // Contact name: max=255
    if (formData.contact_name && formData.contact_name.length > 255) {
      newErrors.contact_name =
        "El nombre de contacto debe tener 255 caracteres o menos";
    }

    // Phone: max=50
    if (formData.phone && formData.phone.length > 50) {
      newErrors.phone = "El teléfono debe tener 50 caracteres o menos";
    }

    // Email: valid format
    if (formData.email) {
      const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
      if (!emailRegex.test(formData.email)) {
        newErrors.email = "El email no tiene un formato válido";
      }
    }

    // Notes: max=1000
    if (formData.notes && formData.notes.length > 1000) {
      newErrors.notes = "Las notas deben tener 1000 caracteres o menos";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    const submitData: CreateSupplierRequest | UpdateSupplierRequest = {
      name: formData.name.trim(),
    };

    if (formData.contact_name.trim()) {
      submitData.contact_name = formData.contact_name.trim();
    }
    if (formData.phone.trim()) {
      submitData.phone = formData.phone.trim();
    }
    if (formData.email.trim()) {
      submitData.email = formData.email.trim();
    }
    if (formData.notes.trim()) {
      submitData.notes = formData.notes.trim();
    }

    await onSubmit(submitData);
  };

  const handleChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  const inputStyle = (hasError: boolean) => ({
    width: "100%",
    padding: "0.75rem",
    border: `1px solid ${hasError ? "var(--color-danger)" : "var(--color-border)"}`,
    borderRadius: "var(--radius-sm)",
    fontSize: "1rem",
    boxSizing: "border-box" as const,
    backgroundColor: "var(--color-bg)",
    color: "var(--color-text-primary)",
  });

  const labelStyle = {
    display: "block",
    marginBottom: "0.5rem",
    fontWeight: "500",
    color: "var(--color-text-primary)",
  };

  const errorStyle = {
    margin: "0.25rem 0 0 0",
    color: "var(--color-danger)",
    fontSize: "0.875rem",
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "var(--color-overlay)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
        padding: "1rem",
      }}
    >
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          padding: "2rem",
          maxWidth: "500px",
          width: "100%",
          maxHeight: "90vh",
          overflowY: "auto",
          boxShadow: "var(--shadow-xl)",
          border: "1px solid var(--color-border)",
        }}
      >
        <h2
          style={{
            marginTop: 0,
            marginBottom: "1.5rem",
            fontSize: "1.5rem",
            fontWeight: "bold",
            color: "var(--color-text-primary)",
          }}
        >
          {supplier ? "Editar Proveedor" : "Nuevo Proveedor"}
        </h2>

        <form onSubmit={handleSubmit}>
          {/* Name */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Nombre del Proveedor *</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => handleChange("name", e.target.value)}
              style={inputStyle(!!errors.name)}
              placeholder="Ingresa el nombre del proveedor"
              maxLength={255}
            />
            {errors.name && <p style={errorStyle}>{errors.name}</p>}
          </div>

          {/* Contact Name */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Nombre de Contacto</label>
            <input
              type="text"
              value={formData.contact_name}
              onChange={(e) => handleChange("contact_name", e.target.value)}
              style={inputStyle(!!errors.contact_name)}
              placeholder="Nombre de la persona de contacto"
              maxLength={255}
            />
            {errors.contact_name && (
              <p style={errorStyle}>{errors.contact_name}</p>
            )}
          </div>

          {/* Phone and Email - Side by Side */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "1rem",
              marginBottom: "1.5rem",
            }}
          >
            <div>
              <label style={labelStyle}>Teléfono</label>
              <input
                type="text"
                value={formData.phone}
                onChange={(e) => handleChange("phone", e.target.value)}
                style={inputStyle(!!errors.phone)}
                placeholder="+57 300 123 4567"
                maxLength={50}
              />
              {errors.phone && <p style={errorStyle}>{errors.phone}</p>}
            </div>

            <div>
              <label style={labelStyle}>Email</label>
              <input
                type="email"
                value={formData.email}
                onChange={(e) => handleChange("email", e.target.value)}
                style={inputStyle(!!errors.email)}
                placeholder="correo@ejemplo.com"
              />
              {errors.email && <p style={errorStyle}>{errors.email}</p>}
            </div>
          </div>

          {/* Notes */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Notas</label>
            <textarea
              value={formData.notes}
              onChange={(e) => handleChange("notes", e.target.value)}
              style={{
                ...inputStyle(!!errors.notes),
                minHeight: "100px",
                resize: "vertical",
              }}
              placeholder="Notas adicionales sobre el proveedor (días de entrega, condiciones, etc.)"
              maxLength={1000}
            />
            {errors.notes && <p style={errorStyle}>{errors.notes}</p>}
          </div>

          {/* Buttons */}
          <div
            style={{ display: "flex", gap: "1rem", justifyContent: "flex-end" }}
          >
            <button
              type="button"
              onClick={onCancel}
              disabled={isLoading}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-surface-hover)",
                color: "var(--color-text-primary)",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                cursor: isLoading ? "not-allowed" : "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isLoading}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-success)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: isLoading ? "not-allowed" : "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              {isLoading ? "Guardando..." : supplier ? "Actualizar" : "Crear"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

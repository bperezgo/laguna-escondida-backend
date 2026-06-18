"use client";

import { useState, useEffect } from "react";
import type {
  Supplier,
  CreateSupplierRequest,
  UpdateSupplierRequest,
} from "@/types/supplier";
import { Modal, Input, Select, Textarea, Button } from "@/components/ui";

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
    identification_type: supplier?.identification_type || "",
    identification_number: supplier?.identification_number || "",
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
        identification_type: supplier.identification_type || "",
        identification_number: supplier.identification_number || "",
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

    // Identification type: max=50
    if (formData.identification_type && formData.identification_type.length > 50) {
      newErrors.identification_type =
        "El tipo de identificación debe tener 50 caracteres o menos";
    }

    // Identification number: max=50
    if (formData.identification_number && formData.identification_number.length > 50) {
      newErrors.identification_number =
        "El número de identificación debe tener 50 caracteres o menos";
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

    if (formData.identification_type.trim()) {
      submitData.identification_type = formData.identification_type.trim();
    }
    if (formData.identification_number.trim()) {
      submitData.identification_number = formData.identification_number.trim();
    }
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

  return (
    <Modal
      open
      onClose={onCancel}
      title={supplier ? "Editar Proveedor" : "Nuevo Proveedor"}
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancelar
          </Button>
          <Button type="submit" form="supplier-form" disabled={isLoading}>
            {isLoading ? "Guardando..." : supplier ? "Actualizar" : "Crear"}
          </Button>
        </>
      }
    >
      <form
        id="supplier-form"
        onSubmit={handleSubmit}
        style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}
      >
        <Input
          label="Nombre del Proveedor *"
          type="text"
          value={formData.name}
          onChange={(e) => handleChange("name", e.target.value)}
          error={errors.name}
          placeholder="Ingresa el nombre del proveedor"
          maxLength={255}
        />

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: "1rem",
          }}
        >
          <Select
            label="Tipo de Identificación"
            value={formData.identification_type}
            onChange={(e) => handleChange("identification_type", e.target.value)}
            error={errors.identification_type}
          >
            <option value="">Seleccionar...</option>
            <option value="NIT">NIT</option>
            <option value="CC">CC (Cédula de Ciudadanía)</option>
            <option value="CE">CE (Cédula de Extranjería)</option>
            <option value="PAS">Pasaporte</option>
            <option value="Otro">Otro</option>
          </Select>

          <Input
            label="Número de Identificación"
            type="text"
            value={formData.identification_number}
            onChange={(e) =>
              handleChange("identification_number", e.target.value)
            }
            error={errors.identification_number}
            placeholder="Ej: 900123456-1"
            maxLength={50}
          />
        </div>

        <Input
          label="Nombre de Contacto"
          type="text"
          value={formData.contact_name}
          onChange={(e) => handleChange("contact_name", e.target.value)}
          error={errors.contact_name}
          placeholder="Nombre de la persona de contacto"
          maxLength={255}
        />

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: "1rem",
          }}
        >
          <Input
            label="Teléfono"
            type="text"
            value={formData.phone}
            onChange={(e) => handleChange("phone", e.target.value)}
            error={errors.phone}
            placeholder="+57 300 123 4567"
            maxLength={50}
          />

          <Input
            label="Email"
            type="email"
            value={formData.email}
            onChange={(e) => handleChange("email", e.target.value)}
            error={errors.email}
            placeholder="correo@ejemplo.com"
          />
        </div>

        <Textarea
          label="Notas"
          value={formData.notes}
          onChange={(e) => handleChange("notes", e.target.value)}
          error={errors.notes}
          placeholder="Notas adicionales sobre el proveedor (días de entrega, condiciones, etc.)"
          maxLength={1000}
        />
      </form>
    </Modal>
  );
}

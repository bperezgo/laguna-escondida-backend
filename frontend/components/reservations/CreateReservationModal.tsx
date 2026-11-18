"use client";

import { FormEvent, useState } from "react";
import { Button, Input, Modal, Select, Textarea } from "@/components/ui";
import { ApiError } from "@/lib/api/config";
import { reservationsApi } from "@/lib/api/reservations";
import { formatSettings } from "@/lib/format";
import { MAX_DAYS_AHEAD, shiftDay, startsAtInZone } from "@/lib/reservations/reservations";
import type { CreateStaffReservationRequest, Reservation } from "@/types/reservation";

type Field = "name" | "customer_phone" | "party_size" | "starts_at" | "notes";

interface CreateReservationModalProps {
  open: boolean;
  onClose: () => void;
  /** The day shown, offered as the booking's day when it is not in the past. */
  day: string;
  today: string;
  onCreated: (reservation: Reservation) => void;
}

const FIELD_MESSAGE: Record<Field, string> = {
  name: "Escribe el nombre (máximo 100 caracteres).",
  customer_phone: "Escribe un teléfono válido, por ejemplo 300 555 1234 o +34 600 123 456.",
  party_size: "Entre 1 y 50 personas.",
  starts_at: `La reserva debe ser en el futuro y a máximo ${MAX_DAYS_AHEAD} días.`,
  notes: "Las notas pueden tener máximo 500 caracteres.",
};

function isField(value: string | undefined): value is Field {
  return value !== undefined && value in FIELD_MESSAGE;
}

export default function CreateReservationModal({ open, onClose, day, today, onCreated }: CreateReservationModalProps) {
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [partySize, setPartySize] = useState("2");
  const [date, setDate] = useState(day < today ? today : day);
  const [time, setTime] = useState("19:00");
  const [notes, setNotes] = useState("");
  const [source, setSource] = useState<CreateStaffReservationRequest["source"]>("phone");
  const [saving, setSaving] = useState(false);
  const [fieldError, setFieldError] = useState<Partial<Record<Field, string>>>({});
  const [formError, setFormError] = useState("");

  const close = () => {
    if (!saving) onClose();
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setFieldError({});
    setFormError("");
    const startsAt = startsAtInZone(date, time, formatSettings.timeZone);
    if (new Date(startsAt).getTime() <= Date.now()) {
      setFieldError({ starts_at: "Esa hora ya pasó." });
      return;
    }
    setSaving(true);
    try {
      const created = await reservationsApi.create({
        customer_phone: phone.trim(),
        name: name.trim(),
        party_size: Number(partySize),
        starts_at: startsAt,
        notes: notes.trim() || undefined,
        source,
      });
      onCreated(created);
    } catch (error) {
      if (error instanceof ApiError && isField(error.body.field)) {
        setFieldError({ [error.body.field]: FIELD_MESSAGE[error.body.field] });
      } else if (error instanceof ApiError && error.status === 403) {
        setFormError("No tienes permiso para crear reservas.");
      } else {
        setFormError("No se pudo crear la reserva. Intenta de nuevo.");
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={close}
      title="Nueva reserva"
      footer={
        <>
          <Button variant="ghost" onClick={close} disabled={saving}>
            Volver
          </Button>
          <Button type="submit" form="create-reservation" disabled={saving}>
            {saving ? "Guardando…" : "Crear reserva"}
          </Button>
        </>
      }
    >
      <form
        id="create-reservation"
        onSubmit={submit}
        style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(12rem, 1fr))", gap: "0.75rem" }}
      >
        <Input
          label="Nombre"
          required
          maxLength={100}
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={fieldError.name}
          autoFocus
        />
        <Input
          label="Teléfono"
          type="tel"
          required
          placeholder="300 555 1234"
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          error={fieldError.customer_phone}
        />
        <Input
          label="Personas"
          type="number"
          required
          min={1}
          max={50}
          value={partySize}
          onChange={(e) => setPartySize(e.target.value)}
          error={fieldError.party_size}
        />
        <Select
          label="¿Cómo llegó?"
          value={source}
          onChange={(e) => setSource(e.target.value === "staff" ? "staff" : "phone")}
        >
          <option value="phone">Llamada</option>
          <option value="staff">Otro (WhatsApp manual, en persona…)</option>
        </Select>
        <Input
          label="Fecha"
          type="date"
          required
          min={today}
          max={shiftDay(today, MAX_DAYS_AHEAD)}
          value={date}
          onChange={(e) => setDate(e.target.value)}
          error={fieldError.starts_at}
        />
        <Input
          label="Hora"
          type="time"
          required
          step={300}
          value={time}
          onChange={(e) => setTime(e.target.value)}
        />
        <div style={{ gridColumn: "1 / -1" }}>
          <Textarea
            label="Notas (opcional)"
            maxLength={500}
            rows={2}
            placeholder="Cumpleaños, silla para bebé…"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            error={fieldError.notes}
          />
        </div>
        <p style={{ gridColumn: "1 / -1", margin: 0, color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
          Queda confirmada de una vez. El cliente no recibe mensaje: avísale tú.
        </p>
        {formError && (
          <p style={{ gridColumn: "1 / -1", margin: 0, color: "var(--color-danger)" }}>{formError}</p>
        )}
      </form>
    </Modal>
  );
}

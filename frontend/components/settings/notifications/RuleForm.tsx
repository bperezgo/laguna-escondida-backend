"use client";

import { useState, type FormEvent } from "react";
import type { NotificationConditions, NotificationEvent, NotificationRule } from "@/types/notifications";
import { NotificationApiError, notificationsApi } from "@/lib/api/notifications";
import {
  CONDITION_LABELS,
  CONFIRMED_BY_LABELS,
  DEFAULT_EXPIRY_MINUTES,
  EVENT_CONDITIONS,
  EVENT_LABELS,
  EVENTS,
  ORDER_TYPE_LABELS,
  SOURCE_LABELS,
  charCount,
  composeTitlePrefix,
  conditionsForEvent,
  expiryLabel,
  freeTextBudget,
  notificationSaveErrors,
  parseTitlePrefix,
  validateRoleId,
  type NotificationFormErrors,
} from "@/lib/settings/notifications";
import { ErrorBanner } from "@/components/deliveries/Banners";
import { Button, Input, Select } from "@/components/ui";
import { muted, row } from "./styles";

interface RuleFormProps {
  channelId: string;
  rule?: NotificationRule;
  onSaved: () => void | Promise<void>;
  onCancel: () => void;
}

const FORM_FIELDS = [
  "event",
  "conditions.order_types",
  "conditions.sources",
  "conditions.confirmed_by",
  "conditions.min_party_size",
  "title_prefix",
  "expires_after_minutes",
];

interface CheckboxGroupProps<K extends string> {
  label: string;
  options: Record<K, string>;
  value: K[] | undefined;
  disabled: boolean;
  error?: string;
  onChange: (value: K[]) => void;
}

function CheckboxGroup<K extends string>({ label, options, value, disabled, error, onChange }: CheckboxGroupProps<K>) {
  const selected = value ?? [];
  return (
    <fieldset style={{ border: "none", padding: 0, margin: 0, display: "flex", flexDirection: "column", gap: "0.35rem" }}>
      <legend style={{ fontWeight: 600, fontSize: "0.9rem", marginBottom: "0.35rem" }}>{label}</legend>
      <div style={row}>
        {(Object.keys(options) as K[]).map((key) => (
          <label key={key} style={{ display: "inline-flex", gap: "0.35rem", alignItems: "center" }}>
            <input
              type="checkbox"
              checked={selected.includes(key)}
              disabled={disabled}
              onChange={(e) => onChange(e.target.checked ? [...selected, key] : selected.filter((v) => v !== key))}
            />
            {options[key]}
          </label>
        ))}
      </div>
      <span style={error ? { color: "var(--color-danger)", fontSize: "0.85rem" } : muted}>
        {error ?? "Sin marcar ninguno, aplica a todos."}
      </span>
    </fieldset>
  );
}

export default function RuleForm({ channelId, rule, onSaved, onCancel }: RuleFormProps) {
  const initialPrefix = parseTitlePrefix(rule?.title_prefix ?? "");
  const initialEvent: NotificationEvent = rule?.event ?? "order.paid";
  const [event, setEvent] = useState<NotificationEvent>(initialEvent);
  const [conditions, setConditions] = useState<NotificationConditions>(rule?.conditions ?? {});
  const [minParty, setMinParty] = useState(rule?.conditions?.min_party_size ? String(rule.conditions.min_party_size) : "");
  const [expiry, setExpiry] = useState(
    rule && rule.expires_after_minutes !== DEFAULT_EXPIRY_MINUTES[rule.event] ? String(rule.expires_after_minutes) : "",
  );
  const [roleId, setRoleId] = useState(initialPrefix.roleId);
  const [text, setText] = useState(initialPrefix.text);
  const [errors, setErrors] = useState<NotificationFormErrors>({ fields: {} });
  const [saving, setSaving] = useState(false);

  const allowed = EVENT_CONDITIONS[event];
  const budget = freeTextBudget(roleId);

  const changeEvent = (next: NotificationEvent) => {
    setEvent(next);
    setConditions({});
    setMinParty("");
    setErrors({ fields: {} });
  };

  const validate = (): Record<string, string> => {
    const fields: Record<string, string> = {};
    const roleError = validateRoleId(roleId);
    if (roleError) fields.role_id = roleError;
    if (charCount(text.trim()) > budget) {
      fields.title_prefix = `El texto puede tener hasta ${budget} caracteres${roleId.trim() ? " junto con la mención del rol" : ""}.`;
    }
    if (minParty.trim() !== "" && !/^[1-9]\d*$/.test(minParty.trim())) {
      fields["conditions.min_party_size"] = "Debe ser un número entero mayor que cero.";
    }
    if (expiry.trim() !== "") {
      const minutes = Number(expiry.trim());
      if (!Number.isInteger(minutes) || minutes < 1 || minutes > 10080) {
        fields.expires_after_minutes = "Debe estar entre 1 y 10080 minutos (7 días).";
      }
    }
    return fields;
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const invalid = validate();
    if (Object.keys(invalid).length > 0) {
      setErrors({ fields: invalid });
      return;
    }
    const body = {
      conditions: conditionsForEvent(event, {
        ...conditions,
        min_party_size: minParty.trim() ? Number(minParty.trim()) : undefined,
      }),
      title_prefix: composeTitlePrefix(roleId, text),
    };
    const minutes = expiry.trim() ? Number(expiry.trim()) : undefined;
    try {
      setSaving(true);
      setErrors({ fields: {} });
      if (rule) {
        // PUT keeps an omitted expiry, so a cleared field sends the event's default explicitly.
        await notificationsApi.updateRule(rule.id, {
          ...body,
          expires_after_minutes: minutes ?? DEFAULT_EXPIRY_MINUTES[rule.event],
        });
      } else {
        await notificationsApi.createRule({
          channel_id: channelId,
          event,
          ...body,
          ...(minutes ? { expires_after_minutes: minutes } : {}),
        });
      }
      await onSaved();
    } catch (err) {
      if (err instanceof NotificationApiError) {
        const mapped = notificationSaveErrors(err.status, err.message, err.code, err.field);
        const unplaced = Object.entries(mapped.fields).find(([field]) => !FORM_FIELDS.includes(field));
        setErrors(unplaced ? { fields: {}, form: unplaced[1] } : mapped);
      } else {
        setErrors({ fields: {}, form: "Error al guardar la regla." });
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      style={{
        padding: "0.75rem",
        border: "1px solid var(--color-border-strong)",
        borderRadius: "var(--radius-sm)",
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
      }}
    >
      {errors.form && <ErrorBanner message={errors.form} />}
      <Select
        label="Evento"
        value={event}
        disabled={saving || Boolean(rule)}
        onChange={(e) => changeEvent(e.target.value as NotificationEvent)}
        error={errors.fields.event}
        helper={rule ? "El evento no se puede cambiar. Para otro evento, crea una regla nueva." : undefined}
      >
        {EVENTS.map((ev) => (
          <option key={ev} value={ev}>
            {EVENT_LABELS[ev]}
          </option>
        ))}
      </Select>

      {allowed.includes("order_types") && (
        <CheckboxGroup
          label={CONDITION_LABELS.order_types}
          options={ORDER_TYPE_LABELS}
          value={conditions.order_types}
          disabled={saving}
          error={errors.fields["conditions.order_types"]}
          onChange={(order_types) => setConditions({ ...conditions, order_types })}
        />
      )}
      {allowed.includes("sources") && (
        <CheckboxGroup
          label={CONDITION_LABELS.sources}
          options={SOURCE_LABELS}
          value={conditions.sources}
          disabled={saving}
          error={errors.fields["conditions.sources"]}
          onChange={(sources) => setConditions({ ...conditions, sources })}
        />
      )}
      {allowed.includes("confirmed_by") && (
        <CheckboxGroup
          label={CONDITION_LABELS.confirmed_by}
          options={CONFIRMED_BY_LABELS}
          value={conditions.confirmed_by}
          disabled={saving}
          error={errors.fields["conditions.confirmed_by"]}
          onChange={(confirmed_by) => setConditions({ ...conditions, confirmed_by })}
        />
      )}
      {allowed.includes("min_party_size") && (
        <Input
          label={CONDITION_LABELS.min_party_size}
          inputMode="numeric"
          placeholder="Cualquier tamaño"
          value={minParty}
          disabled={saving}
          onChange={(e) => setMinParty(e.target.value)}
          error={errors.fields["conditions.min_party_size"]}
          helper="Avisa solo para reservas de al menos estas personas."
        />
      )}

      <Input
        label="Vence en (minutos)"
        inputMode="numeric"
        placeholder={String(DEFAULT_EXPIRY_MINUTES[event])}
        value={expiry}
        disabled={saving}
        onChange={(e) => setExpiry(e.target.value)}
        error={errors.fields.expires_after_minutes}
        helper={`Vacío usa el valor por defecto: ${expiryLabel(DEFAULT_EXPIRY_MINUTES[event])}. Un aviso que no se entrega a tiempo se descarta.`}
      />

      <Input
        label="ID del rol de Discord a mencionar (opcional)"
        inputMode="numeric"
        spellCheck={false}
        placeholder="123456789012345678"
        value={roleId}
        disabled={saving}
        onChange={(e) => setRoleId(e.target.value)}
        error={errors.fields.role_id}
        helper="En Discord, con el modo desarrollador activo: Ajustes del servidor → Roles → clic derecho en el rol → Copiar ID."
      />
      <Input
        label="Texto al inicio del aviso (opcional)"
        placeholder="Atención"
        value={text}
        disabled={saving}
        onChange={(e) => setText(e.target.value)}
        error={errors.fields.title_prefix}
        helper={`Hasta ${budget} caracteres.`}
      />

      <div style={row}>
        <Button type="submit" size="sm" disabled={saving}>
          {saving ? "Guardando…" : rule ? "Guardar regla" : "Agregar regla"}
        </Button>
        <Button type="button" size="sm" variant="secondary" onClick={onCancel} disabled={saving}>
          Cancelar
        </Button>
      </div>
    </form>
  );
}

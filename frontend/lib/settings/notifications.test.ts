import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import type { NotificationChannel, NotificationRule } from "../../types/notifications.ts";
import {
  CONDITION_LABELS,
  CONFIRMED_BY_LABELS,
  EVENT_CONDITIONS,
  EVENT_LABELS,
  EVENTS,
  ORDER_TYPE_LABELS,
  SOURCE_LABELS,
  channelTestResult,
  composeTitlePrefix,
  conditionsForEvent,
  freeTextBudget,
  hasNotificationErrors,
  notificationSaveErrors,
  notificationsPageState,
  parseTitlePrefix,
  ruleSummary,
  validateRoleId,
  webhookHint,
} from "./notifications.ts";

const apiDoc = readFileSync(
  new URL("../../../backend/docs/api/notification-channels.md", import.meta.url),
  "utf8",
);

function docCatalogue(): Record<string, string[]> {
  const rows: Record<string, string[]> = {};
  const section = apiDoc.slice(apiDoc.indexOf("## Event catalogue"));
  for (const line of section.split("\n")) {
    const cells = line.split("|").map((c) => c.trim());
    const event = cells[1]?.match(/^`([a-z]+\.[a-z]+)`$/)?.[1];
    if (event) rows[event] = Array.from(cells[3].matchAll(/`([a-z_]+)`/g), (m) => m[1]);
  }
  return rows;
}

function docValues(condition: string): string[] {
  const line = apiDoc.split("\n").find((l) => l.startsWith(`- \`${condition}\`:`)) ?? "";
  const values = line.slice(line.indexOf(":") + 1).split(" (")[0];
  return Array.from(values.matchAll(/`([a-z_]+)`/g), (m) => m[1]);
}

const channel: NotificationChannel = {
  id: "c1",
  name: "Domicilios",
  type: "discord",
  enabled: true,
  settings: { webhook_url_hint: "…1234" },
  created_at: "2026-11-18T14:00:00Z",
  updated_at: "2026-11-18T14:00:00Z",
};

function rule(overrides: Partial<NotificationRule>): NotificationRule {
  return {
    id: "r1",
    channel_id: "c1",
    event: "order.paid",
    conditions: {},
    title_prefix: "",
    expires_after_minutes: 60,
    enabled: true,
    created_at: "2026-11-18T14:05:00Z",
    updated_at: "2026-11-18T14:05:00Z",
    ...overrides,
  };
}

test("every event and value has its Spanish label", () => {
  assert.deepEqual(EVENT_LABELS, {
    "order.created": "Pedido creado",
    "order.paid": "Pedido pagado",
    "reservation.requested": "Reserva por confirmar",
    "reservation.confirmed": "Reserva confirmada",
  });
  assert.deepEqual(ORDER_TYPE_LABELS, { dine_in: "En mesa", takeaway: "Para llevar", delivery: "Domicilio" });
  assert.deepEqual(SOURCE_LABELS, {
    whatsapp: "WhatsApp",
    phone: "Teléfono",
    walk_in: "Sin reserva",
    staff: "Personal",
  });
  assert.deepEqual(CONFIRMED_BY_LABELS, { staff: "Por el personal", auto: "Automáticamente" });
  assert.equal(CONDITION_LABELS.min_party_size, "Mínimo de personas");
  assert.equal(CONDITION_LABELS.confirmed_by, "Confirmada por");
});

test("each event's conditions match the API doc's catalogue", () => {
  const catalogue = docCatalogue();
  assert.deepEqual(Object.keys(catalogue).sort(), [...EVENTS].sort());
  for (const event of EVENTS) {
    assert.deepEqual([...EVENT_CONDITIONS[event]].sort(), [...catalogue[event]].sort(), event);
  }
});

test("each condition value in the API doc has a label", () => {
  assert.deepEqual(Object.keys(ORDER_TYPE_LABELS).sort(), docValues("order_types").sort());
  assert.deepEqual(Object.keys(SOURCE_LABELS).sort(), docValues("sources").sort());
  assert.deepEqual(Object.keys(CONFIRMED_BY_LABELS).sort(), docValues("confirmed_by").sort());
});

test("a rule reads as its event and labelled conditions", () => {
  assert.equal(ruleSummary(rule({ conditions: { order_types: ["delivery"] } })), "Pedido pagado · Domicilio");
  assert.equal(
    ruleSummary(rule({ event: "reservation.requested", conditions: { sources: ["whatsapp"], min_party_size: 6 } })),
    "Reserva por confirmar · WhatsApp · Mínimo de personas: 6",
  );
  assert.equal(ruleSummary(rule({ event: "order.created" })), "Pedido creado");
});

test("the webhook hint is masked", () => {
  assert.equal(webhookHint("…1234"), "••••1234");
});

test("only the event's conditions are sent", () => {
  const all = { order_types: ["delivery" as const], sources: ["phone" as const], confirmed_by: ["auto" as const], min_party_size: 4 };
  assert.deepEqual(conditionsForEvent("order.paid", all), { order_types: ["delivery"] });
  assert.deepEqual(conditionsForEvent("reservation.requested", all), { sources: ["phone"], min_party_size: 4 });
  assert.deepEqual(conditionsForEvent("reservation.confirmed", all), {
    sources: ["phone"],
    confirmed_by: ["auto"],
    min_party_size: 4,
  });
  assert.deepEqual(conditionsForEvent("order.created", { order_types: [] }), {});
});

test("compose and parse round-trip a role mention with text", () => {
  const prefix = composeTitlePrefix("123456789", "Atención");
  assert.equal(prefix, "<@&123456789> Atención");
  assert.deepEqual(parseTitlePrefix(prefix), { roleId: "123456789", text: "Atención" });
});

test("compose and parse round-trip a role mention alone", () => {
  const prefix = composeTitlePrefix(" 42 ", "  ");
  assert.equal(prefix, "<@&42>");
  assert.deepEqual(parseTitlePrefix(prefix), { roleId: "42", text: "" });
});

test("compose and parse round-trip text without a role", () => {
  const prefix = composeTitlePrefix("", " Urgente ");
  assert.equal(prefix, "Urgente");
  assert.deepEqual(parseTitlePrefix(prefix), { roleId: "", text: "Urgente" });
});

test("a prefix that is not a role mention is all free text", () => {
  assert.deepEqual(parseTitlePrefix("<@123> hola"), { roleId: "", text: "<@123> hola" });
  assert.deepEqual(parseTitlePrefix("Hola <@&123>"), { roleId: "", text: "Hola <@&123>" });
  assert.deepEqual(parseTitlePrefix(""), { roleId: "", text: "" });
});

test("a role ID must be digits only", () => {
  assert.match(validateRoleId("abc") ?? "", /solo dígitos/);
  assert.ok(validateRoleId("12a"));
  assert.equal(validateRoleId("123456789"), undefined);
  assert.equal(validateRoleId(""), undefined);
});

test("the role mention takes its share of the 100 characters", () => {
  assert.equal(freeTextBudget(""), 100);
  assert.equal(freeTextBudget("123456789"), 100 - "<@&123456789> ".length);
  const text = "a".repeat(freeTextBudget("123456789"));
  assert.equal(composeTitlePrefix("123456789", text).length, 100);
});

test("a server without a notification key is unavailable", () => {
  const state = notificationsPageState({
    channels: { status: 503, code: "NOTIFICATIONS_DISABLED" },
    rules: { status: 503, code: "NOTIFICATIONS_DISABLED" },
  });
  assert.deepEqual(state, { kind: "unavailable" });
});

test("load failures are errors", () => {
  for (const status of [403, 404, 500, 503]) {
    const state = notificationsPageState({ channels: { status }, rules: { status: 200, data: [] } });
    assert.equal(state.kind, "error", `status ${status}`);
  }
  const rulesFailed = notificationsPageState({ channels: { status: 200, data: [] }, rules: { status: 500 } });
  assert.equal(rulesFailed.kind, "error");
});

test("rules are grouped under their channel", () => {
  const other: NotificationChannel = { ...channel, id: "c2", name: "Cocina" };
  const r1 = rule({ id: "r1" });
  const r2 = rule({ id: "r2", event: "reservation.requested" });
  const orphan = rule({ id: "r3", channel_id: "gone" });
  const state = notificationsPageState({
    channels: { status: 200, data: [channel, other] },
    rules: { status: 200, data: [r1, r2, orphan] },
  });
  assert.deepEqual(state, { kind: "ready", channels: [channel, other], rulesByChannel: { c1: [r1, r2], c2: [] } });
});

test("no channels is ready and empty", () => {
  const state = notificationsPageState({ channels: { status: 200, data: [] }, rules: { status: 200, data: [] } });
  assert.deepEqual(state, { kind: "ready", channels: [], rulesByChannel: {} });
});

test("a refused field is shown next to that field", () => {
  const webhook = notificationSaveErrors(
    400,
    "settings.webhook_url: must be an https Discord webhook URL",
    "NOTIFICATION_CHANNEL_INVALID_FIELD",
    "settings.webhook_url",
  );
  assert.deepEqual(Object.keys(webhook.fields), ["settings.webhook_url"]);
  assert.equal(webhook.form, undefined);
  assert.equal(hasNotificationErrors(webhook), true);

  const duplicate = notificationSaveErrors(400, "name: is already used by another channel", undefined, "name");
  assert.match(duplicate.fields.name, /Ya hay un canal/);

  const condition = notificationSaveErrors(400, "is not a condition of this event", undefined, "conditions.min_party_size");
  assert.ok(condition.fields["conditions.min_party_size"]);
});

test("refusals without a field are the form's message", () => {
  assert.match(notificationSaveErrors(400, "bad json").form ?? "", /no aceptó/);
  assert.match(notificationSaveErrors(503, "", "NOTIFICATIONS_DISABLED").form ?? "", /no están habilitadas/);
  assert.match(notificationSaveErrors(403, "").form ?? "", /administrador/);
  assert.equal(hasNotificationErrors({ fields: {} }), false);
});

test("a test send reports success or Discord's status", () => {
  assert.equal(channelTestResult(204).ok, true);
  const refused = channelTestResult(502, "NOTIFICATION_CHANNEL_UNREACHABLE", 404);
  assert.equal(refused.ok, false);
  assert.match(refused.message, /no aceptó/);
  assert.match(refused.message, /404/);
  assert.match(channelTestResult(502, "NOTIFICATION_CHANNEL_UNREACHABLE").message, /No se pudo contactar/);
  assert.match(channelTestResult(403).message, /administrador/);
  assert.match(channelTestResult(500).message, /prueba/);
});

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Badge, Button, Modal, Textarea } from "@/components/ui";
import type { BadgeTone } from "@/components/ui";
import { ApiError } from "@/lib/api/config";
import { reservationsApi } from "@/lib/api/reservations";
import { whatsappLink } from "@/lib/feedback/feedback";
import { formatDate, formatSettings, formatTime, inBusinessTimeZone, todayInBusinessTz } from "@/lib/format";
import { useNow } from "@/lib/kitchen/useNow";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import {
  FILTER_LABEL,
  OVERDUE_DAYS,
  SOURCE_LABEL,
  STATUS_LABEL,
  actionsFor,
  applyActiveChange,
  applyChange,
  applyPendingChange,
  businessDayOf,
  isOverdue,
  matchesFilter,
  shiftDay,
  sortReservations,
  startsAtInZone,
  summarize,
  type StatusFilter,
} from "@/lib/reservations/reservations";
import type { Reservation, ReservationAction, ReservationStatus } from "@/types/reservation";
import CreateReservationModal from "./CreateReservationModal";
import { useReservationStream, type StreamState } from "./useReservationStream";
import styles from "./Reservations.module.css";

const STATUS_TONE: Record<ReservationStatus, BadgeTone> = {
  requested: "warning",
  confirmed: "info",
  visited: "success",
  declined: "neutral",
  cancelled: "neutral",
  no_show: "danger",
};

const ACTION_LABEL: Record<ReservationAction, string> = {
  confirm: "Confirmar",
  decline: "Rechazar",
  cancel: "Cancelar",
  "no-show": "No llegó",
};

const STREAM_LABEL: Record<StreamState, string> = {
  live: "En vivo",
  connecting: "Conectando…",
  offline: "Sin conexión, reintentando",
};

const FILTERS: StatusFilter[] = ["all", "requested", "confirmed", "visited", "closed"];
const ACTIVE_FILTERS: StatusFilter[] = ["all", "requested", "confirmed"];

export type ReservationsView = "day" | "all";

const VIEWS: ReservationsView[] = ["day", "all"];

const VIEW_LABEL: Record<ReservationsView, string> = {
  day: "Por día",
  all: "Pendientes y confirmadas",
};
const FRESH_MS = 8000;

interface PendingAction {
  reservation: Reservation;
  action: Exclude<ReservationAction, "confirm">;
}

function dayLabel(day: string, today: string): string {
  const date = formatDate(`${day}T12:00:00Z`, {
    weekday: "long",
    day: "numeric",
    month: "long",
    timeZone: "UTC",
  });
  if (day === today) return `Hoy, ${date}`;
  if (day === shiftDay(today, 1)) return `Mañana, ${date}`;
  if (day === shiftDay(today, -1)) return `Ayer, ${date}`;
  return date;
}

// On a request whose time has passed, confirming records that the guest came.
function actionLabel(reservation: Reservation, action: ReservationAction, now: number): string {
  return action === "confirm" && isOverdue(reservation, now) ? "Llegó" : ACTION_LABEL[action];
}

function queueDayLabel(day: string, today: string): string {
  if (day === today) return "Hoy";
  if (day === shiftDay(today, 1)) return "Mañana";
  if (day === shiftDay(today, -1)) return "Ayer";
  return formatDate(`${day}T12:00:00Z`, { weekday: "short", day: "numeric", month: "short", timeZone: "UTC" });
}

function actionError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) return "La reserva ya había cambiado. La lista se actualizó.";
    if (error.status === 403) return "No tienes permiso para cambiar reservas.";
    if (error.status === 404) return "La reserva ya no existe. La lista se actualizó.";
    return error.message;
  }
  return "No se pudo cambiar la reserva. Intenta de nuevo.";
}

function cancelledByLabel(reservation: Reservation): string | null {
  if (reservation.status !== "cancelled" || !reservation.cancelled_by) return null;
  return reservation.cancelled_by === "customer" ? "Cancelada por el cliente" : "Cancelada por el restaurante";
}

export default function ReservationsPageClient({
  requestedDay,
  requestedView,
}: {
  requestedDay: string | null;
  requestedView: ReservationsView;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const { hasPermission, isAdmin, isLoading: permissionsLoading } = usePermissions();
  const canManage = hasPermission(PERMISSIONS.RESERVATIONS_MANAGE);
  // Bookings come from the agent; creating them here is the admins' fallback, kept off the floor.
  const canCreate = canManage && isAdmin;
  const now = useNow(30_000);
  const today = todayInBusinessTz(new Date(now));

  const [view, setView] = useState<ReservationsView>(requestedView);
  const [day, setDay] = useState(requestedDay ?? today);
  const [reservations, setReservations] = useState<Reservation[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [filter, setFilter] = useState<StatusFilter>("all");
  const [busyId, setBusyId] = useState<string | null>(null);
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [declineReason, setDeclineReason] = useState("");
  const [fresh, setFresh] = useState<Set<string>>(new Set());
  const [creating, setCreating] = useState(false);
  const [createKey, setCreateKey] = useState(0);
  const [pendingQueue, setPendingQueue] = useState<Reservation[]>([]);
  const [active, setActive] = useState<Reservation[]>([]);
  const [activeLoading, setActiveLoading] = useState(true);
  const [activeError, setActiveError] = useState("");
  const [activeFilter, setActiveFilter] = useState<StatusFilter>("all");

  const dayRef = useRef(day);
  const todayRef = useRef(today);
  const viewRef = useRef(view);
  const pendingSeq = useRef(0);
  const activeSeq = useRef(0);
  const loadSeq = useRef(0);
  const freshTimers = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  const load = useCallback(async (target: string) => {
    const seq = ++loadSeq.current;
    try {
      const data = await reservationsApi.listDay(target);
      if (seq !== loadSeq.current) return;
      setReservations(sortReservations(data.reservations));
      setLoadError("");
    } catch (error) {
      if (seq !== loadSeq.current) return;
      setLoadError(
        error instanceof ApiError && error.status === 403
          ? "No tienes permiso para ver las reservas."
          : "No se pudieron cargar las reservas."
      );
    } finally {
      if (seq === loadSeq.current) setLoading(false);
    }
  }, []);

  // Requests the agent took for any day, so staff needn't browse each day to find them.
  const loadPending = useCallback(async () => {
    const seq = ++pendingSeq.current;
    try {
      const from = startsAtInZone(shiftDay(todayRef.current, -OVERDUE_DAYS), "00:00", formatSettings.timeZone);
      const data = await reservationsApi.listPending(from);
      if (seq === pendingSeq.current) setPendingQueue(sortReservations(data.reservations));
    } catch {
      // The day list shows the load error; the queue keeps what it had.
    }
  }, []);

  // Every pending and confirmed reservation, whatever day it is for; overdue requests included.
  const loadActive = useCallback(async () => {
    const seq = ++activeSeq.current;
    try {
      const from = startsAtInZone(shiftDay(todayRef.current, -OVERDUE_DAYS), "00:00", formatSettings.timeZone);
      const data = await reservationsApi.listActive(from);
      if (seq !== activeSeq.current) return;
      setActive(sortReservations(data.reservations));
      setActiveError("");
    } catch (error) {
      if (seq !== activeSeq.current) return;
      setActiveError(
        error instanceof ApiError && error.status === 403
          ? "No tienes permiso para ver las reservas."
          : "No se pudieron cargar las reservas."
      );
    } finally {
      if (seq === activeSeq.current) setActiveLoading(false);
    }
  }, []);

  useEffect(() => {
    todayRef.current = today;
    if (!permissionsLoading && canManage) void loadPending();
  }, [today, permissionsLoading, canManage, loadPending]);

  useEffect(() => {
    viewRef.current = view;
    if (view === "all") void loadActive();
  }, [view, today, loadActive]);

  useEffect(() => {
    dayRef.current = day;
    void load(day);
  }, [day, load]);

  useEffect(() => {
    const timers = freshTimers.current;
    return () => timers.forEach(clearTimeout);
  }, []);

  const markFresh = useCallback((id: string) => {
    setFresh((prev) => new Set(prev).add(id));
    const timers = freshTimers.current;
    const previous = timers.get(id);
    if (previous) clearTimeout(previous);
    timers.set(
      id,
      setTimeout(() => {
        timers.delete(id);
        setFresh((prev) => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
      }, FRESH_MS)
    );
  }, []);

  const onChange = useCallback(
    (changed: Reservation) => {
      setReservations((prev) => applyChange(prev, changed, dayRef.current));
      setPendingQueue((prev) => applyPendingChange(prev, changed, shiftDay(todayRef.current, -OVERDUE_DAYS)));
      setActive((prev) => applyActiveChange(prev, changed, shiftDay(todayRef.current, -OVERDUE_DAYS)));
      if (changed.status === "requested") markFresh(changed.id);
    },
    [markFresh]
  );
  const onOpen = useCallback(() => {
    void load(dayRef.current);
    void loadPending();
    if (viewRef.current === "all") void loadActive();
  }, [load, loadPending, loadActive]);
  const stream = useReservationStream(!permissionsLoading && canManage, onChange, onOpen);

  const syncUrl = (nextView: ReservationsView, nextDay: string) => {
    const query = new URLSearchParams();
    if (nextView === "all") query.set("view", "all");
    else if (nextDay !== today) query.set("date", nextDay);
    router.replace(query.size > 0 ? `${pathname}?${query}` : pathname, { scroll: false });
  };

  const switchView = (next: ReservationsView) => {
    if (next === view) return;
    setView(next);
    setActionMessage("");
    syncUrl(next, day);
  };

  // Also leaves the "all" view, so a date picked from it opens that day.
  const goTo = (target: string) => {
    if (!target || (target === day && view === "day")) return;
    setActionMessage("");
    setView("day");
    syncUrl("day", target);
    if (target === day) return;
    setLoading(true);
    setReservations([]);
    setDay(target);
  };

  const run = async (reservation: Reservation, action: ReservationAction, reason?: string) => {
    setBusyId(reservation.id);
    setActionMessage("");
    try {
      const updated = await reservationsApi.act(reservation.id, action, reason);
      setReservations((prev) => applyChange(prev, updated, dayRef.current));
      setPendingQueue((prev) => applyPendingChange(prev, updated, shiftDay(todayRef.current, -OVERDUE_DAYS)));
      setActive((prev) => applyActiveChange(prev, updated, shiftDay(todayRef.current, -OVERDUE_DAYS)));
      setPending(null);
    } catch (error) {
      setActionMessage(actionError(error));
      setPending(null);
      void load(dayRef.current);
      void loadPending();
      if (viewRef.current === "all") void loadActive();
    } finally {
      setBusyId(null);
    }
  };

  const onCreated = (created: Reservation) => {
    setCreating(false);
    setFilter("all");
    markFresh(created.id);
    setActive((prev) => applyActiveChange(prev, created, shiftDay(todayRef.current, -OVERDUE_DAYS)));
    if (view === "all") return;
    const createdDay = businessDayOf(created.starts_at);
    if (createdDay === day) {
      setReservations((prev) => applyChange(prev, created, day));
    } else {
      goTo(createdDay);
    }
  };

  const openCreate = () => {
    setCreateKey((k) => k + 1);
    setCreating(true);
  };

  const onAction = (reservation: Reservation, action: ReservationAction) => {
    if (action === "confirm") {
      void run(reservation, action);
      return;
    }
    setDeclineReason("");
    setPending({ reservation, action });
  };

  const summary = useMemo(() => summarize(reservations), [reservations]);
  const counts = useMemo(
    () =>
      Object.fromEntries(
        FILTERS.map((f) => [f, reservations.filter((r) => matchesFilter(r, f)).length])
      ) as Record<StatusFilter, number>,
    [reservations]
  );
  const shown = reservations.filter((r) => matchesFilter(r, filter));
  const overdue = pendingQueue.filter((r) => isOverdue(r, now));
  const upcoming = pendingQueue.filter((r) => !isOverdue(r, now));
  const activeCounts = useMemo(
    () =>
      Object.fromEntries(
        ACTIVE_FILTERS.map((f) => [f, active.filter((r) => matchesFilter(r, f)).length])
      ) as Record<StatusFilter, number>,
    [active]
  );
  const activeShown = active.filter((r) => matchesFilter(r, activeFilter));
  const timeFormat = inBusinessTimeZone({ hour: "2-digit", minute: "2-digit" });

  const renderQueueItem = (reservation: Reservation) => {
    const reservationDay = businessDayOf(reservation.starts_at);
    const busy = busyId === reservation.id;
    const late = isOverdue(reservation, now);
    const actions: ReservationAction[] = late ? ["confirm", "no-show"] : ["confirm", "decline"];
    return (
      <li
        key={reservation.id}
        className={styles.queueItem}
        data-fresh={fresh.has(reservation.id)}
        data-current={reservationDay === day}
      >
        <button type="button" className={styles.queueWhen} onClick={() => goTo(reservationDay)} title="Ver ese día">
          <span className={styles.queueDay}>{queueDayLabel(reservationDay, today)}</span>
          <span>{formatTime(reservation.starts_at, timeFormat)}</span>
        </button>
        <div className={styles.queueWho}>
          <span className={styles.name}>{reservation.name}</span>
          <span className={styles.queueMeta}>
            {reservation.party_size} {reservation.party_size === 1 ? "persona" : "personas"} ·{" "}
            <a href={`tel:${reservation.contact_phone}`}>{reservation.contact_phone}</a>
            {late && (
              <>
                {" · "}
                <a href={whatsappLink(reservation.contact_phone)} target="_blank" rel="noopener noreferrer">
                  WhatsApp
                </a>
              </>
            )}
          </span>
          {reservation.notes && <span className={styles.note}>“{reservation.notes}”</span>}
        </div>
        <div className={styles.actions}>
          {actions.map((action) => (
            <Button
              key={action}
              size="sm"
              variant={action === "confirm" ? "primary" : action === "no-show" ? "danger" : "secondary"}
              disabled={busy}
              onClick={() => onAction(reservation, action)}
            >
              {busy && action === "confirm" ? "Guardando…" : actionLabel(reservation, action, now)}
            </Button>
          ))}
        </div>
      </li>
    );
  };

  const renderItem = (reservation: Reservation, withDay: boolean) => {
    const actions = canManage ? actionsFor(reservation, now) : [];
    const busy = busyId === reservation.id;
    const cancelledBy = cancelledByLabel(reservation);
    return (
      <article
        key={reservation.id}
        className={styles.item}
        data-status={reservation.status}
        data-fresh={fresh.has(reservation.id)}
      >
        <div>
          {withDay && (
            <button
              type="button"
              className={styles.itemDay}
              onClick={() => goTo(businessDayOf(reservation.starts_at))}
              title="Ver ese día"
            >
              {queueDayLabel(businessDayOf(reservation.starts_at), today)}
            </button>
          )}
          <div className={styles.time}>{formatTime(reservation.starts_at, timeFormat)}</div>
          <div className={styles.party}>
            {reservation.party_size} {reservation.party_size === 1 ? "persona" : "personas"}
          </div>
        </div>

        <div className={styles.main}>
          <div className={styles.nameRow}>
            <span className={styles.name}>{reservation.name}</span>
            <Badge tone={STATUS_TONE[reservation.status]}>{STATUS_LABEL[reservation.status]}</Badge>
            {reservation.origin.attribution?.ad_id && (
              <Badge tone="info" dot={false}>
                Anuncio
              </Badge>
            )}
          </div>
          <div className={styles.meta}>
            <a href={`tel:${reservation.contact_phone}`}>{reservation.contact_phone}</a>
            <span>{SOURCE_LABEL[reservation.source]}</span>
            {cancelledBy && <span>{cancelledBy}</span>}
          </div>
          {reservation.notes && <div className={styles.note}>“{reservation.notes}”</div>}
          {reservation.status === "declined" && reservation.decline_reason && (
            <div className={styles.note}>Motivo: {reservation.decline_reason}</div>
          )}
        </div>

        {actions.length > 0 && (
          <div className={styles.actions}>
            {actions.map((action) => (
              <Button
                key={action}
                size="sm"
                variant={action === "confirm" ? "primary" : action === "no-show" ? "danger" : "secondary"}
                disabled={busy}
                onClick={() => onAction(reservation, action)}
              >
                {busy && action === "confirm" ? "Guardando…" : actionLabel(reservation, action, now)}
              </Button>
            ))}
          </div>
        )}
      </article>
    );
  };

  return (
    <div className={styles.page}>
      <div className={styles.inner}>
        <div>
          <h1 className={styles.title}>Reservas</h1>
          <p className={styles.subtitle}>
            Las reservas del día, las que llegan por WhatsApp y lo que pasó con cada una.
          </p>
        </div>

        <div className={styles.tabs} role="tablist" aria-label="Vista de reservas">
          {VIEWS.map((v) => (
            <button
              key={v}
              type="button"
              role="tab"
              className={styles.tab}
              aria-selected={view === v}
              onClick={() => switchView(v)}
            >
              {VIEW_LABEL[v]}
            </button>
          ))}
        </div>

        <div className={styles.toolbar}>
          {view === "day" ? (
              <div className={styles.dayNav}>
                <Button variant="secondary" size="sm" onClick={() => goTo(shiftDay(day, -1))} aria-label="Día anterior">
                  ←
                </Button>
                <span className={styles.dayLabel} aria-live="polite">
                  {dayLabel(day, today)}
                </span>
                <Button variant="secondary" size="sm" onClick={() => goTo(shiftDay(day, 1))} aria-label="Día siguiente">
                  →
                </Button>
                {day !== today && (
                  <Button variant="ghost" size="sm" onClick={() => goTo(today)}>
                    Hoy
                  </Button>
                )}
                <input
                  type="date"
                  className={styles.dateInput}
                  value={day}
                  onChange={(e) => goTo(e.target.value)}
                  aria-label="Elegir fecha"
                />
              </div>
          ) : (
            <span className={styles.subtitle}>Todas las solicitudes y reservas confirmadas, sin importar el día.</span>
          )}
          <div className={styles.dayNav}>
            {canCreate && <Button onClick={openCreate}>Nueva reserva</Button>}
            {canManage && (
              <span className={styles.live} title="Las reservas nuevas y sus cambios aparecen solos">
                <span className={styles.liveDot} data-state={stream} />
                {STREAM_LABEL[stream]}
              </span>
            )}
          </div>
        </div>

        {view === "day" ? (
          <>
            {canManage && overdue.length > 0 && (
              <section className={styles.queue} data-kind="overdue" aria-label="Solicitudes sin respuesta">
                <div className={styles.queueHead}>
                  <h2 className={styles.queueTitle}>Sin respuesta</h2>
                  <span className={styles.queueCount}>
                    Ya pasó la hora y nadie las contestó. Marca qué pasó; el cliente no recibe ningún mensaje.
                  </span>
                </div>
                <ul className={styles.queueList}>{overdue.map(renderQueueItem)}</ul>
              </section>
            )}

            {canManage && upcoming.length > 0 && (
              <section className={styles.queue} aria-label="Solicitudes por confirmar">
                <div className={styles.queueHead}>
                  <h2 className={styles.queueTitle}>Por confirmar</h2>
                  <span className={styles.queueCount}>
                    {upcoming.length} {upcoming.length === 1 ? "solicitud" : "solicitudes"} de todos los días
                  </span>
                </div>
                <ul className={styles.queueList}>{upcoming.map(renderQueueItem)}</ul>
              </section>
            )}

            <div className={styles.summary}>
              <div className={styles.stat}>
                <span className={styles.statValue}>{summary.reservations}</span>
                <span className={styles.statLabel}>Reservas</span>
              </div>
              <div className={styles.stat}>
                <span className={styles.statValue}>{summary.pending}</span>
                <span className={styles.statLabel}>Por confirmar</span>
              </div>
              <div className={styles.stat}>
                <span className={styles.statValue}>{summary.guests}</span>
                <span className={styles.statLabel}>Personas confirmadas</span>
              </div>
            </div>

            <div className={styles.filters} role="group" aria-label="Filtrar por estado">
              {FILTERS.map((f) => (
                <button
                  key={f}
                  type="button"
                  className={styles.filter}
                  aria-pressed={filter === f}
                  onClick={() => setFilter(f)}
                >
                  {FILTER_LABEL[f]} ({counts[f]})
                </button>
              ))}
            </div>

            {loadError && <div className={styles.banner}>{loadError}</div>}
            {actionMessage && <div className={styles.banner}>{actionMessage}</div>}

            <div className={styles.list}>
              {!loading && !loadError && shown.length === 0 && (
                <div className={styles.empty}>
                  {reservations.length === 0 ? "No hay reservas para este día." : "Ninguna reserva con este estado."}
                </div>
              )}
              {loading && reservations.length === 0 && <div className={styles.empty}>Cargando reservas…</div>}
              {shown.map((reservation) => renderItem(reservation, false))}
            </div>
          </>
        ) : (
          <>
            <div className={styles.filters} role="group" aria-label="Filtrar por estado">
              {ACTIVE_FILTERS.map((f) => (
                <button
                  key={f}
                  type="button"
                  className={styles.filter}
                  aria-pressed={activeFilter === f}
                  onClick={() => setActiveFilter(f)}
                >
                  {FILTER_LABEL[f]} ({activeCounts[f]})
                </button>
              ))}
            </div>

            {activeError && <div className={styles.banner}>{activeError}</div>}
            {actionMessage && <div className={styles.banner}>{actionMessage}</div>}

            <div className={styles.list}>
              {!activeLoading && !activeError && activeShown.length === 0 && (
                <div className={styles.empty}>
                  {active.length === 0
                    ? "No hay reservas pendientes ni confirmadas."
                    : "Ninguna reserva con este estado."}
                </div>
              )}
              {activeLoading && active.length === 0 && <div className={styles.empty}>Cargando reservas…</div>}
              {activeShown.map((reservation) => renderItem(reservation, true))}
            </div>
          </>
        )}
      </div>

      {canCreate && creating && (
        <CreateReservationModal
          key={createKey}
          open
          onClose={() => setCreating(false)}
          day={day}
          today={today}
          onCreated={onCreated}
        />
      )}

      <Modal
        open={pending !== null}
        onClose={() => busyId === null && setPending(null)}
        size="sm"
        title={
          pending?.action === "decline"
            ? "Rechazar la solicitud"
            : pending?.action === "cancel"
              ? "Cancelar la reserva"
              : "Marcar que no llegó"
        }
        footer={
          pending && (
            <>
              <Button variant="ghost" onClick={() => setPending(null)} disabled={busyId !== null}>
                Volver
              </Button>
              <Button
                variant={pending.action === "decline" ? "primary" : "danger"}
                disabled={busyId !== null}
                onClick={() =>
                  void run(pending.reservation, pending.action, declineReason.trim() || undefined)
                }
              >
                {busyId !== null ? "Guardando…" : ACTION_LABEL[pending.action]}
              </Button>
            </>
          )
        }
      >
        {pending && (
          <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
            <p style={{ margin: 0 }}>
              {pending.reservation.name}, {pending.reservation.party_size}{" "}
              {pending.reservation.party_size === 1 ? "persona" : "personas"}, a las{" "}
              {formatTime(pending.reservation.starts_at, timeFormat)}.
            </p>
            {pending.action === "decline" && (
              <Textarea
                label="Motivo (opcional)"
                helper={
                  pending.reservation.source === "whatsapp"
                    ? "El cliente recibe un mensaje por WhatsApp."
                    : undefined
                }
                maxLength={255}
                rows={3}
                value={declineReason}
                onChange={(e) => setDeclineReason(e.target.value)}
              />
            )}
            {pending.action === "cancel" && pending.reservation.source === "whatsapp" && !isOverdue(pending.reservation, now) && (
              <p style={{ margin: 0, color: "var(--color-text-secondary)" }}>
                El cliente recibe un mensaje por WhatsApp.
              </p>
            )}
            {pending.action === "no-show" && (
              <p style={{ margin: 0, color: "var(--color-text-secondary)" }}>
                No se le pedirá su opinión ni recibe ningún mensaje. No se puede deshacer.
              </p>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}

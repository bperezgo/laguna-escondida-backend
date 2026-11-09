"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Badge, Button } from "@/components/ui";
import { ApiError } from "@/lib/api/config";
import { feedbackApi } from "@/lib/api/feedback";
import { formatDate, formatDateTime, formatNumber, inBusinessTimeZone, todayInBusinessTz } from "@/lib/format";
import {
  RATING_FILTER_LABEL,
  matchesRating,
  rangeFor,
  shiftRange,
  summarizeFeedback,
  whatsappLink,
  type Period,
  type Range,
  type RatingFilter,
} from "@/lib/feedback/feedback";
import type { Feedback } from "@/types/feedback";
import styles from "./Feedback.module.css";

const FILTERS: RatingFilter[] = ["all", "attention", "good"];
const LONG_FORMAT: Intl.DateTimeFormatOptions = {
  day: "numeric",
  month: "long",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
};

function dayText(day: string, options: Intl.DateTimeFormatOptions): string {
  return formatDate(`${day}T12:00:00Z`, { ...options, timeZone: "UTC" });
}

function rangeLabel(range: Range, period: Period): string {
  if (period === "month") return dayText(range.from, { month: "long", year: "numeric" });
  return `${dayText(range.from, { day: "numeric", month: "short" })} – ${dayText(range.to, {
    day: "numeric",
    month: "short",
    year: "numeric",
  })}`;
}

function Stars({ rating }: { rating: number }) {
  return (
    <span className={styles.stars} role="img" aria-label={`${rating} de 5`}>
      {[1, 2, 3, 4, 5].map((n) => (
        <span key={n} data-on={n <= rating}>
          ★
        </span>
      ))}
    </span>
  );
}

interface FeedbackPageClientProps {
  requestedDay: string | null;
  requestedPeriod: Period;
}

export default function FeedbackPageClient({ requestedDay, requestedPeriod }: FeedbackPageClientProps) {
  const router = useRouter();
  const pathname = usePathname();
  const today = todayInBusinessTz();

  const [period, setPeriod] = useState<Period>(requestedPeriod);
  const [range, setRange] = useState<Range>(() => rangeFor(requestedDay ?? today, requestedPeriod));
  const [items, setItems] = useState<Feedback[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [filter, setFilter] = useState<RatingFilter>("all");
  const loadSeq = useRef(0);

  const load = useCallback(async (target: Range) => {
    const seq = ++loadSeq.current;
    try {
      const data = await feedbackApi.list(target.from, target.to);
      if (seq !== loadSeq.current) return;
      setItems(data.feedback);
      setLoadError("");
    } catch (error) {
      if (seq !== loadSeq.current) return;
      setItems([]);
      setLoadError(
        error instanceof ApiError && error.status === 403
          ? "No tienes permiso para ver las opiniones."
          : "No se pudieron cargar las opiniones."
      );
    } finally {
      if (seq === loadSeq.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(range);
  }, [range, load]);

  const current = rangeFor(today, period);
  const isCurrent = range.from === current.from;

  const show = (target: Range, targetPeriod: Period) => {
    if (target.from === range.from && targetPeriod === period) return;
    setLoading(true);
    setPeriod(targetPeriod);
    setRange(target);
    const query = new URLSearchParams();
    if (targetPeriod === "month") query.set("period", "month");
    if (target.from !== rangeFor(today, targetPeriod).from) query.set("date", target.from);
    router.replace(query.size > 0 ? `${pathname}?${query}` : pathname, { scroll: false });
  };

  const summary = useMemo(() => summarizeFeedback(items), [items]);
  const counts = useMemo(
    () =>
      Object.fromEntries(FILTERS.map((f) => [f, items.filter((i) => matchesRating(i, f)).length])) as Record<
        RatingFilter,
        number
      >,
    [items]
  );
  const shown = items.filter((i) => matchesRating(i, filter));
  const maxBar = Math.max(1, ...summary.distribution);
  const visitFormat = inBusinessTimeZone({ weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
  const answeredFormat = inBusinessTimeZone({ day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

  return (
    <div className={styles.page}>
      <div className={styles.inner}>
        <div>
          <h1 className={styles.title}>Opiniones</h1>
          <p className={styles.subtitle}>
            Lo que dicen los clientes al día siguiente de su reserva, cuando se les pregunta por WhatsApp.
          </p>
        </div>

        <div className={styles.toolbar}>
          <div className={styles.rangeNav}>
            <Button variant="secondary" size="sm" onClick={() => show(shiftRange(range, period, -1), period)} aria-label="Anterior">
              ←
            </Button>
            <span className={styles.rangeLabel} aria-live="polite">
              {rangeLabel(range, period)}
            </span>
            <Button variant="secondary" size="sm" onClick={() => show(shiftRange(range, period, 1), period)} aria-label="Siguiente">
              →
            </Button>
            {!isCurrent && (
              <Button variant="ghost" size="sm" onClick={() => show(current, period)}>
                {period === "week" ? "Esta semana" : "Este mes"}
              </Button>
            )}
          </div>
          <div className={styles.segmented} role="group" aria-label="Periodo">
            {(["week", "month"] as const).map((p) => (
              <button
                key={p}
                type="button"
                aria-pressed={period === p}
                onClick={() => show(rangeFor(range.from === current.from ? today : range.from, p), p)}
              >
                {p === "week" ? "Semana" : "Mes"}
              </button>
            ))}
          </div>
        </div>

        <div className={styles.summary}>
          <div className={styles.stat}>
            <span className={styles.statValue}>
              {summary.average === null ? "—" : formatNumber(summary.average, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}
            </span>
            <span className={styles.statLabel}>Calificación promedio</span>
            {summary.average !== null && <Stars rating={Math.round(summary.average)} />}
          </div>
          <div className={styles.stat}>
            <span className={styles.statValue}>{summary.total}</span>
            <span className={styles.statLabel}>Opiniones</span>
          </div>
          <div className={styles.stat} data-alert={summary.attention > 0}>
            <span className={styles.statValue}>{summary.attention}</span>
            <span className={styles.statLabel}>Requieren atención (1–3)</span>
          </div>
          <div className={`${styles.stat} ${styles.distribution}`} aria-label="Opiniones por calificación">
            {[5, 4, 3, 2, 1].map((stars) => {
              const count = summary.distribution[stars - 1];
              return (
                <div key={stars} className={styles.barRow}>
                  <span className={styles.barLabel}>{stars}★</span>
                  <span className={styles.barTrack}>
                    <span
                      className={styles.barFill}
                      data-low={stars <= 3}
                      style={{ width: `${(count / maxBar) * 100}%` }}
                    />
                  </span>
                  <span className={styles.barCount}>{count}</span>
                </div>
              );
            })}
          </div>
        </div>

        <div className={styles.filters} role="group" aria-label="Filtrar por calificación">
          {FILTERS.map((f) => (
            <button key={f} type="button" className={styles.filter} aria-pressed={filter === f} onClick={() => setFilter(f)}>
              {RATING_FILTER_LABEL[f]} ({counts[f]})
            </button>
          ))}
        </div>

        {loadError && <div className={styles.banner}>{loadError}</div>}

        <div className={styles.list}>
          {loading && items.length === 0 && <div className={styles.empty}>Cargando opiniones…</div>}
          {!loading && !loadError && shown.length === 0 && (
            <div className={styles.empty}>
              {items.length === 0 ? "No hay opiniones en este periodo." : "Ninguna opinión con este filtro."}
            </div>
          )}
          {shown.map((item) => (
            <article key={item.id} className={styles.item} data-attention={item.needs_attention}>
              <div className={styles.head}>
                <Stars rating={item.rating} />
                <span className={styles.name}>{item.reservation.name}</span>
                {item.needs_attention && <Badge tone="danger">Requiere atención</Badge>}
                <span className={styles.answered} title={`Respondida ${formatDateTime(item.updated_at, inBusinessTimeZone(LONG_FORMAT))}`}>
                  {formatDateTime(item.updated_at, answeredFormat)}
                </span>
              </div>
              {item.comment ? (
                <p className={styles.comment}>“{item.comment}”</p>
              ) : (
                <p className={styles.noComment}>Sin comentario</p>
              )}
              <div className={styles.meta}>
                <span>
                  Visita: {formatDateTime(item.reservation.starts_at, visitFormat)} · {item.reservation.party_size}{" "}
                  {item.reservation.party_size === 1 ? "persona" : "personas"}
                </span>
                <a href={`tel:${item.contact_phone}`}>{item.contact_phone}</a>
                <a href={whatsappLink(item.contact_phone)} target="_blank" rel="noopener noreferrer">
                  Escribir por WhatsApp
                </a>
              </div>
            </article>
          ))}
        </div>
      </div>
    </div>
  );
}

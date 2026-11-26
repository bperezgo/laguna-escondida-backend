"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ChartLineUp, Receipt, ShoppingCart, Storefront, Ticket, Wallet } from "@phosphor-icons/react";
import { Button, StatCard } from "@/components/ui";
import { dailyCloseApi } from "@/lib/api/dailyClose";
import { financialSummaryApi } from "@/lib/api/financialSummary";
import { dayFigures, type DayFigures } from "@/lib/dashboard/figures";
import { formatDate, formatMoney, formatNumber, inBusinessTimeZone } from "@/lib/format";
import { usePermissions } from "@/lib/permissions";
import { PERMISSIONS } from "@/lib/permissions/constants";
import styles from "./Dashboard.module.css";

type State =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; figures: DayFigures };

export default function Dashboard() {
  const { hasPermission } = usePermissions();
  const [state, setState] = useState<State>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  const retry = useCallback(() => {
    setState({ status: "loading" });
    setAttempt((value) => value + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const close = await dailyCloseApi.getDailyClose("");
        // The close's own bounds, so both requests describe the same business day.
        const summary = await financialSummaryApi
          .getSummaryBetween(close.start_date, close.end_date)
          .catch(() => null);
        if (!cancelled) setState({ status: "ready", figures: dayFigures(close, summary) });
      } catch {
        if (!cancelled) setState({ status: "error" });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  if (state.status === "loading") {
    return (
      <div className={styles.page}>
        <p className={styles.status} role="status">
          Cargando las cifras de hoy…
        </p>
      </div>
    );
  }

  if (state.status === "error") {
    return (
      <div className={styles.page}>
        <div className={styles.notice} role="alert">
          <h2 className={styles.noticeTitle}>No pudimos cargar las cifras de hoy</h2>
          <p className={styles.noticeText}>Revisa tu conexión e inténtalo de nuevo.</p>
          <Button onClick={retry}>Reintentar</Button>
        </div>
      </div>
    );
  }

  const { figures } = state;
  const day = formatDate(
    figures.day,
    inBusinessTimeZone({ weekday: "long", day: "numeric", month: "long" })
  );
  const canInstallBox = hasPermission(PERMISSIONS.EDGE_ENROLL);

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Hoy</p>
          <h2 className={styles.day}>{day}</h2>
        </div>
        <nav aria-label="Reportes" className={styles.links}>
          <Link href="/close" className={styles.link}>
            Cierre de caja
          </Link>
          <Link href="/financial" className={styles.link}>
            Finanzas
          </Link>
        </nav>
      </header>

      {figures.isEmpty ? (
        <section className={styles.empty} aria-labelledby="dashboard-empty">
          <span className={styles.emptyIcon} aria-hidden="true">
            <Storefront size={28} />
          </span>
          <h3 id="dashboard-empty" className={styles.noticeTitle}>
            Todavía no hay ventas hoy
          </h3>
          <p className={styles.noticeText}>
            Aquí verás las ventas del día a medida que se cobren las órdenes en la caja del restaurante.
          </p>
          {canInstallBox && (
            <Link href="/ajustes/instalar-caja" className={styles.emptyAction}>
              Instalar la caja
            </Link>
          )}
        </section>
      ) : (
        <>
          <section className={styles.stats} aria-label="Ventas de hoy">
            <StatCard
              label="Ventas"
              value={formatMoney(figures.collected)}
              icon={<ChartLineUp size={20} aria-hidden="true" />}
            />
            <StatCard
              label="Órdenes pagadas"
              value={formatNumber(figures.orders)}
              icon={<Receipt size={20} aria-hidden="true" />}
            />
            <StatCard
              label="Ticket promedio"
              value={figures.averageTicket === null ? "—" : formatMoney(figures.averageTicket)}
              icon={<Ticket size={20} aria-hidden="true" />}
            />
          </section>

          {figures.paymentMethods.length > 0 && (
            <section className={styles.panel} aria-labelledby="dashboard-methods">
              <h3 id="dashboard-methods" className={styles.panelTitle}>
                Por método de pago
              </h3>
              <ul className={styles.methods}>
                {figures.paymentMethods.map((method) => (
                  <li key={method.method} className={styles.method}>
                    <span className={styles.methodLabel}>{method.label}</span>
                    <span className={styles.methodCount}>
                      {formatNumber(method.count)} {method.count === 1 ? "pago" : "pagos"}
                    </span>
                    <span className={styles.methodAmount}>{formatMoney(method.collected)}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}

      {figures.spending ? (
        <section className={styles.stats} aria-label="Gastos y compras de hoy">
          <StatCard
            label="Gastos"
            value={formatMoney(figures.spending.expenses)}
            icon={<Wallet size={20} aria-hidden="true" />}
          />
          <StatCard
            label="Compras"
            value={formatMoney(figures.spending.purchases)}
            icon={<ShoppingCart size={20} aria-hidden="true" />}
          />
        </section>
      ) : (
        <div className={styles.inlineNotice} role="alert">
          <span>No pudimos cargar los gastos y las compras de hoy.</span>
          <Button variant="secondary" size="sm" onClick={retry}>
            Reintentar
          </Button>
        </div>
      )}
    </div>
  );
}

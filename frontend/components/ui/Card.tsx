import { HTMLAttributes, ReactNode } from "react";
import styles from "./Card.module.css";

function cx(...parts: (string | false | undefined)[]) {
  return parts.filter(Boolean).join(" ");
}

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
}

export function Card({ interactive, className = "", children, ...rest }: CardProps) {
  return (
    <div className={cx(styles.card, interactive && styles.interactive, className)} {...rest}>
      {children}
    </div>
  );
}

export function CardHeader({
  className = "",
  children,
  actions,
  ...rest
}: HTMLAttributes<HTMLDivElement> & { actions?: ReactNode }) {
  return (
    <div className={cx(styles.header, className)} {...rest}>
      <div className={styles.headerMain}>{children}</div>
      {actions && <div className={styles.headerActions}>{actions}</div>}
    </div>
  );
}

export function CardTitle({ className = "", children, ...rest }: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3 className={cx(styles.title, className)} {...rest}>
      {children}
    </h3>
  );
}

export function CardSubtitle({
  className = "",
  children,
  ...rest
}: HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p className={cx(styles.subtitle, className)} {...rest}>
      {children}
    </p>
  );
}

export function CardBody({ className = "", children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cx(styles.body, className)} {...rest}>
      {children}
    </div>
  );
}

export function CardFooter({ className = "", children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cx(styles.footer, className)} {...rest}>
      {children}
    </div>
  );
}

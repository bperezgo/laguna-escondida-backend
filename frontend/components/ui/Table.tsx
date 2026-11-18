import { TableHTMLAttributes } from "react";
import styles from "./Table.module.css";

export interface TableProps extends TableHTMLAttributes<HTMLTableElement> {
  /** Wrapper className (the scroll container) */
  wrapperClassName?: string;
}

/**
 * Styled data table. Use plain <thead>/<tbody>/<tr>/<th>/<td> inside.
 * Add `data-numeric` to a <th>/<td> for right-aligned tabular (monospace) numbers.
 */
export function Table({ className = "", wrapperClassName = "", children, ...rest }: TableProps) {
  return (
    <div className={[styles.wrap, wrapperClassName].filter(Boolean).join(" ")}>
      <table className={[styles.table, className].filter(Boolean).join(" ")} {...rest}>
        {children}
      </table>
    </div>
  );
}

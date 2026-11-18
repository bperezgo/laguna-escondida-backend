"use client";

import {
  InputHTMLAttributes,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
  ReactNode,
  forwardRef,
  useId,
} from "react";
import styles from "./field.module.css";

function cx(...parts: (string | false | undefined)[]) {
  return parts.filter(Boolean).join(" ");
}

interface FieldShellProps {
  label?: ReactNode;
  helper?: ReactNode;
  error?: ReactNode;
  fieldId: string;
  className?: string;
  children: ReactNode;
}

function FieldShell({ label, helper, error, fieldId, className = "", children }: FieldShellProps) {
  return (
    <div className={cx(styles.field, !!error && styles.error, className)}>
      {label && (
        <label htmlFor={fieldId} className={styles.label}>
          {label}
        </label>
      )}
      {children}
      {(error || helper) && <div className={styles.helper}>{error || helper}</div>}
    </div>
  );
}

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: ReactNode;
  helper?: ReactNode;
  error?: ReactNode;
  wrapperClassName?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { label, helper, error, id, className = "", wrapperClassName, ...rest },
  ref
) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  return (
    <FieldShell
      label={label}
      helper={helper}
      error={error}
      fieldId={fieldId}
      className={wrapperClassName}
    >
      <input id={fieldId} ref={ref} className={cx(styles.control, className)} {...rest} />
    </FieldShell>
  );
});

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: ReactNode;
  helper?: ReactNode;
  error?: ReactNode;
  wrapperClassName?: string;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { label, helper, error, id, className = "", wrapperClassName, children, ...rest },
  ref
) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  return (
    <FieldShell
      label={label}
      helper={helper}
      error={error}
      fieldId={fieldId}
      className={wrapperClassName}
    >
      <select
        id={fieldId}
        ref={ref}
        className={cx(styles.control, styles.select, className)}
        {...rest}
      >
        {children}
      </select>
    </FieldShell>
  );
});

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: ReactNode;
  helper?: ReactNode;
  error?: ReactNode;
  wrapperClassName?: string;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { label, helper, error, id, className = "", wrapperClassName, rows = 4, ...rest },
  ref
) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  return (
    <FieldShell
      label={label}
      helper={helper}
      error={error}
      fieldId={fieldId}
      className={wrapperClassName}
    >
      <textarea
        id={fieldId}
        ref={ref}
        rows={rows}
        className={cx(styles.control, styles.textarea, className)}
        {...rest}
      />
    </FieldShell>
  );
});

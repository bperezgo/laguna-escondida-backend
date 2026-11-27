"use client";

import { forwardRef, useCallback, useLayoutEffect, useReducer, useRef } from "react";
import { Input, type InputProps } from "./Field";
import { inputType, passwordVisibilityReducer, toggleLabel } from "@/lib/passwordVisibility";
import styles from "./field.module.css";

export interface PasswordInputProps extends Omit<InputProps, "type"> {
  /** Controlled visibility; omit to let the field keep its own. */
  shown?: boolean;
  onShownChange?: (shown: boolean) => void;
  /** Follow `shown` without rendering a toggle (a confirm field sharing the password's toggle). */
  hideToggle?: boolean;
}

function EyeIcon({ off }: { off: boolean }) {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" />
      <circle cx="12" cy="12" r="3" />
      {off && <path d="M3 3l18 18" />}
    </svg>
  );
}

const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(function PasswordInput(
  { shown, onShownChange, hideToggle = false, className = "", disabled, ...rest },
  forwardedRef
) {
  const [ownShown, dispatch] = useReducer(passwordVisibilityReducer, false);
  const isShown = shown ?? ownShown;
  const inputRef = useRef<HTMLInputElement | null>(null);
  const selection = useRef<{ start: number | null; end: number | null } | null>(null);

  const setRefs = useCallback(
    (node: HTMLInputElement | null) => {
      inputRef.current = node;
      if (typeof forwardedRef === "function") forwardedRef(node);
      else if (forwardedRef) forwardedRef.current = node;
    },
    [forwardedRef]
  );

  useLayoutEffect(() => {
    const saved = selection.current;
    const input = inputRef.current;
    if (!saved || !input) return;
    selection.current = null;
    input.focus();
    if (saved.start !== null && saved.end !== null) input.setSelectionRange(saved.start, saved.end);
  }, [isShown]);

  const toggle = () => {
    const input = inputRef.current;
    selection.current = { start: input?.selectionStart ?? null, end: input?.selectionEnd ?? null };
    if (shown === undefined) dispatch("toggled");
    else onShownChange?.(!isShown);
  };

  return (
    <Input
      {...rest}
      ref={setRefs}
      type={inputType(isShown)}
      disabled={disabled}
      className={`${hideToggle ? "" : styles.passwordControl} ${className}`}
      endAdornment={
        hideToggle ? undefined : (
          <button
            type="button"
            className={styles.passwordToggle}
            onClick={toggle}
            disabled={disabled}
            aria-label={toggleLabel(isShown)}
            aria-pressed={isShown}
          >
            <EyeIcon off={isShown} />
          </button>
        )
      }
    />
  );
});

export default PasswordInput;

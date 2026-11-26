"use client";

import { useEffect, useRef } from "react";
import { googleClientId } from "@/lib/google";

interface GoogleCredentialResponse {
  credential?: string;
}

interface GoogleIdentity {
  accounts: {
    id: {
      initialize(options: { client_id: string; callback: (response: GoogleCredentialResponse) => void }): void;
      renderButton(element: HTMLElement, options: Record<string, unknown>): void;
    };
  };
}

declare global {
  interface Window {
    google?: GoogleIdentity;
  }
}

const GIS_SRC = "https://accounts.google.com/gsi/client";
let gisLoading: Promise<GoogleIdentity> | null = null;

function loadGoogleIdentity(): Promise<GoogleIdentity> {
  if (window.google?.accounts?.id) return Promise.resolve(window.google);
  if (!gisLoading) {
    gisLoading = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = GIS_SRC;
      script.async = true;
      script.defer = true;
      script.onload = () => (window.google ? resolve(window.google) : reject(new Error("GIS unavailable")));
      script.onerror = () => {
        gisLoading = null;
        reject(new Error("GIS failed to load"));
      };
      document.head.appendChild(script);
    });
  }
  return gisLoading;
}

export interface GoogleSignInButtonProps {
  onCredential: (idToken: string) => void;
  text?: "signin_with" | "signup_with" | "continue_with";
  disabled?: boolean;
}

export default function GoogleSignInButton({ onCredential, text = "signin_with", disabled = false }: GoogleSignInButtonProps) {
  const clientId = googleClientId();
  const containerRef = useRef<HTMLDivElement>(null);
  const callbackRef = useRef(onCredential);

  useEffect(() => {
    callbackRef.current = onCredential;
  }, [onCredential]);

  useEffect(() => {
    if (!clientId) return;
    let cancelled = false;
    loadGoogleIdentity()
      .then((google) => {
        if (cancelled || !containerRef.current) return;
        google.accounts.id.initialize({
          client_id: clientId,
          callback: (response) => {
            if (response.credential) callbackRef.current(response.credential);
          },
        });
        google.accounts.id.renderButton(containerRef.current, {
          theme: "outline",
          size: "large",
          text,
          shape: "rectangular",
          locale: "es",
          width: containerRef.current.offsetWidth || 320,
        });
      })
      .catch((error) => console.error("Google sign-in unavailable:", error));
    return () => {
      cancelled = true;
    };
  }, [clientId, text]);

  if (!clientId) return null;

  return (
    <div
      ref={containerRef}
      aria-disabled={disabled}
      style={{
        width: "100%",
        minHeight: "44px",
        display: "flex",
        justifyContent: "center",
        opacity: disabled ? 0.5 : 1,
        pointerEvents: disabled ? "none" : undefined,
      }}
    />
  );
}

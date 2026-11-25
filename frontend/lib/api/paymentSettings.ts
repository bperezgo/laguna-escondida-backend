import type { PaymentSettings, PaymentSettingsRequest } from "@/types/paymentSettings";

/** Keeps the backend's status and field, e.g. 400 on manual_instructions. */
export class PaymentSettingsError extends Error {
  status: number;
  field?: string;

  constructor(message: string, status: number, field?: string) {
    super(message);
    this.name = "PaymentSettingsError";
    this.status = status;
    this.field = field;
  }
}

export const paymentSettingsApi = {
  async save(settings: PaymentSettingsRequest): Promise<PaymentSettings> {
    const response = await fetch("/api/organization/payment-settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(settings),
    });
    if (response.status === 401) {
      window.location.href = "/signin";
      throw new PaymentSettingsError("Unauthorized", 401);
    }
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new PaymentSettingsError(body.error || `HTTP error! status: ${response.status}`, response.status, body.field);
    }
    return response.json();
  },
};

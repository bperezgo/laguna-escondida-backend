export type PaymentStrategy = "manual" | "wompi";

export interface PaymentSettings {
  strategy: PaymentStrategy;
  manual_instructions: string;
  manual_window_minutes: number;
}

export interface PaymentSettingsRequest {
  strategy: PaymentStrategy;
  manual_instructions: string;
  manual_window_minutes: number;
}

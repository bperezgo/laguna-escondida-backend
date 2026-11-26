import { apiRequest } from "./config";

export interface EnrollmentCode {
  code: string;
  expires_at: string;
}

export interface GeneratedPassword {
  username: string;
  password: string;
}

export const installBoxApi = {
  issueCode(): Promise<EnrollmentCode> {
    return apiRequest<EnrollmentCode>("/edge/enrollment-codes", { method: "POST" });
  },

  generatePassword(userId: string): Promise<GeneratedPassword> {
    return apiRequest<GeneratedPassword>(`/admin/users/${encodeURIComponent(userId)}/generate-password`, {
      method: "POST",
    });
  },
};

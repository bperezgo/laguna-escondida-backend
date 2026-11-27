import { apiRequest } from "./config";

export interface EnrollmentCode {
  code: string;
  expires_at: string;
}

export const installBoxApi = {
  issueCode(): Promise<EnrollmentCode> {
    return apiRequest<EnrollmentCode>("/edge/enrollment-codes", { method: "POST" });
  },
};

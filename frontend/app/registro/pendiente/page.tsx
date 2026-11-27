import { redirect } from "next/navigation";
import { PAYMENT_PATH } from "@/lib/signup";

// The old waiting page: its content is now step 2's status page.
export default function SignupPendingPage() {
  redirect(PAYMENT_PATH);
}

import SignInForm from "@/components/auth/SignInForm";
import { config } from "@/lib/config/config";
import { parseSignInContext, signInNotice, type SignInContext } from "@/lib/signIn";

export const dynamic = "force-dynamic";

// Read here so the page opens in the right layout: two panels in the cloud, one on a box.
async function signInContext(): Promise<SignInContext | null> {
  try {
    const response = await fetch(`${config.apiUrl}/auth/context`, { cache: "no-store" });
    return response.ok ? parseSignInContext(await response.json()) : null;
  } catch {
    return null;
  }
}

export default async function SignInPage({ searchParams }: { searchParams: Promise<{ aviso?: string | string[] }> }) {
  const [{ aviso }, context] = await Promise.all([searchParams, signInContext()]);
  return <SignInForm initialContext={context} notice={signInNotice(aviso)} />;
}

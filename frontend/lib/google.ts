export function googleClientId(raw: string | undefined = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID): string | null {
  const id = raw?.trim();
  return id ? id : null;
}

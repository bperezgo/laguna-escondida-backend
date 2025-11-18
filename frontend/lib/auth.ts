import { cookies } from 'next/headers';

/**
 * Check if user is authenticated by verifying access_token cookie exists
 */
export async function isAuthenticated(): Promise<boolean> {
  const cookieStore = await cookies();
  const token = cookieStore.get('access_token');
  return !!token?.value;
}

/**
 * Get the access token from cookies (server-side only)
 */
export async function getAccessToken(): Promise<string | undefined> {
  const cookieStore = await cookies();
  return cookieStore.get('access_token')?.value;
}


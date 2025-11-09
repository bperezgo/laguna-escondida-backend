import { config } from '@/lib/config/config';

/**
 * Server-side API client that calls the Golang backend directly
 * This should only be used in Next.js API routes and server components
 */
export async function serverApiRequest<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const url = `${config.apiUrl}${endpoint}`;
  
  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: response.statusText }));
    throw new Error(error.message || `HTTP error! status: ${response.status}`);
  }

  return response.json();
}


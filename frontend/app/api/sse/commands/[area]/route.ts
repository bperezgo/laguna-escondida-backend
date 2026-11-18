import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { cookies } from "next/headers";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ area: string }> }
) {
  const { area } = await params;

  const token = await getAccessToken();

  if (!token) {
    const cookieStore = await cookies();
    cookieStore.delete("access_token");
    return new Response(JSON.stringify({ error: "Unauthorized" }), {
      status: 401,
      headers: { "Content-Type": "application/json" },
    });
  }

  const backendUrl = `${config.apiUrl}/sse/commands/${area}`;

  try {
    const response = await fetch(backendUrl, {
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });

    if (!response.ok) {
      if (response.status === 401) {
        const cookieStore = await cookies();
        cookieStore.delete("access_token");
      }
      return new Response(
        JSON.stringify({ error: `Backend error: ${response.status}` }),
        {
          status: response.status,
          headers: { "Content-Type": "application/json" },
        }
      );
    }

    if (!response.body) {
      return new Response(JSON.stringify({ error: "No response body" }), {
        status: 500,
        headers: { "Content-Type": "application/json" },
      });
    }

    const { readable, writable } = new TransformStream();

    response.body.pipeTo(writable).catch((err) => {
      console.error("SSE pipe error:", err);
    });

    return new Response(readable, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache, no-store, must-revalidate",
        Connection: "keep-alive",
        "X-Accel-Buffering": "no",
      },
    });
  } catch (error) {
    console.error("SSE connection error:", error);
    return new Response(JSON.stringify({ error: "Failed to connect to SSE" }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    });
  }
}

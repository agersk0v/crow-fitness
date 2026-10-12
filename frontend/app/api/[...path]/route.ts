import type { NextRequest } from "next/server";

export const dynamic = "force-dynamic";

// Server-only transport. Workout rules remain in Go.
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const endpoint = path.join("/");
  if (!new Set(["health", "ready"]).has(endpoint)) {
    return Response.json(
      { error: { code: "not_found", message: "Not found." } },
      { status: 404 },
    );
  }

  try {
    const base = process.env.API_INTERNAL_URL ?? "http://127.0.0.1:8080";
    const upstream = await fetch(new URL(`/api/${endpoint}`, base), {
      cache: "no-store",
      signal: AbortSignal.timeout(4000),
    });
    return new Response(await upstream.text(), {
      status: upstream.status,
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-store",
      },
    });
  } catch {
    return Response.json(
      {
        error: {
          code: "api_unavailable",
          message: "The workout API could not be reached.",
        },
      },
      { status: 502, headers: { "Cache-Control": "no-store" } },
    );
  }
}

import type { NextRequest } from "next/server";

export const dynamic = "force-dynamic";

// Server-only transport. Workout rules remain in Go.
async function forward(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const endpoint = path.join("/");
  const resource =
    path.length === 2 &&
    path[0] === "exercises" &&
    /^[1-9][0-9]*$/.test(path[1]);
  const usage =
    path.length === 3 &&
    path[0] === "exercises" &&
    /^[1-9][0-9]*$/.test(path[1]) &&
    path[2] === "templates";
  const template =
    path.length === 2 &&
    path[0] === "templates" &&
    /^[1-9][0-9]*$/.test(path[1]);
  const templateAction =
    path.length === 3 &&
    path[0] === "templates" &&
    /^[1-9][0-9]*$/.test(path[1]) &&
    ["duplicate", "archive", "restore"].includes(path[2]);
  const allowed =
    (request.method === "GET" &&
      (["health", "ready", "exercises", "templates"].includes(endpoint) ||
        usage ||
        template)) ||
    (request.method === "POST" &&
      (["exercises", "templates"].includes(endpoint) || templateAction)) ||
    (["PUT", "DELETE"].includes(request.method) && resource) ||
    (request.method === "PUT" && template);
  if (!allowed)
    return Response.json(
      { error: { code: "not_found", message: "Not found." } },
      { status: 404 },
    );
  const hasBody = ["POST", "PUT"].includes(request.method);

  try {
    const base = process.env.API_INTERNAL_URL ?? "http://127.0.0.1:8080";
    const upstream = await fetch(
      new URL(`/api/${endpoint}${request.nextUrl.search}`, base),
      {
        cache: "no-store",
        method: request.method,
        headers: hasBody
          ? { "Content-Type": request.headers.get("content-type") ?? "" }
          : undefined,
        body: hasBody ? request.body : undefined,
        ...(hasBody ? { duplex: "half" } : {}),
        signal: AbortSignal.timeout(4000),
      },
    );
    return new Response(
      upstream.status === 204 ? null : await upstream.text(),
      {
        status: upstream.status,
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "no-store",
          ...(upstream.headers.get("Location")
            ? { Location: upstream.headers.get("Location")! }
            : {}),
        },
      },
    );
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

export const GET = forward;
export const POST = forward;

export const PUT = forward;
export const DELETE = forward;

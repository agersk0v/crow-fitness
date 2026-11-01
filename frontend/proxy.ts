import { NextRequest, NextResponse } from "next/server";

export async function proxy(request: NextRequest) {
  const response = await fetch(
    new URL(
      "/api/auth/me",
      process.env.API_INTERNAL_URL ?? "http://127.0.0.1:8080",
    ),
    {
      headers: {
        Cookie: request.cookies
          .getAll()
          .filter(({ name }) =>
            ["__Host-crow_session", "crow_session_dev"].includes(name),
          )
          .map(({ name, value }) => `${name}=${value}`)
          .join("; "),
      },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    },
  ).catch(() => null);
  if (response?.status === 401)
    return NextResponse.redirect(new URL("/login", request.url));
  if (!response?.ok)
    return new NextResponse(
      "Sign-in is temporarily unavailable. Please try again.",
      { status: 503, headers: { "Cache-Control": "private, no-store" } },
    );
  const next = NextResponse.next();
  next.headers.set("Cache-Control", "private, no-store");
  return next;
}
export const config = {
  matcher: ["/((?!api(?:/|$)|login(?:/|$)|_next/|favicon.ico).*)"],
};

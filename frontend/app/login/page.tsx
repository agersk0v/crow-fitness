"use client";
import { useEffect } from "react";
import { useAuth } from "@/components/auth-boundary";
import { Button } from "@/components/ui/button";
export default function LoginPage() {
  const { openLogin } = useAuth();
  // A protected-page redirect or failed callback opens the same provider chooser.
  useEffect(() => {
    openLogin();
  }, [openLogin]);
  return (
    <main className="mx-auto max-w-lg px-6 py-16">
      <h1 className="text-3xl font-semibold">Log in to view your workouts</h1>
      <p className="mt-4 text-muted-foreground">
        Your exercises, templates, and history are private to your account.
      </p>
      <Button className="mt-6" onClick={openLogin}>
        Log in
      </Button>
    </main>
  );
}

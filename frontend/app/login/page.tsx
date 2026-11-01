import { Login } from "@/components/auth-boundary";
export default function LoginPage() {
  return (
    <main className="mx-auto max-w-lg px-6 py-16">
      <h1 className="text-3xl font-semibold">Sign in to Crow Fitness</h1>
      <p className="mt-4 text-muted-foreground">
        Your exercises, workouts, and history belong to your account.
      </p>
      <Login />
    </main>
  );
}

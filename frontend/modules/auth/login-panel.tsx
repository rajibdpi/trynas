"use client";

import { HardDrive, Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";

export function LoginPanel({
  username, password, setUsername, setPassword, loginError,
  signing, dark, toggleTheme, signIn,
}: {
  username: string;
  password: string;
  setUsername: (value: string) => void;
  setPassword: (value: string) => void;
  loginError: string;
  signing: boolean;
  dark: boolean;
  toggleTheme: () => void;
  signIn: () => Promise<void>;
}) {
  return (
      <main className="login-screen grid min-h-screen place-items-center p-4">
        <Card className="w-full max-w-md">
          <CardHeader>
            <div className="flex items-center gap-2 text-xl font-bold text-primary">
              <HardDrive />
              TryNAS <span className="text-xs">V5</span>
            </div>
            <CardTitle className="mt-4 text-2xl">Welcome back</CardTitle>
            <p className="text-sm text-muted-foreground">
              Sign in to your private NAS.
            </p>
          </CardHeader>
          <CardContent>
            <form
              className="space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                void signIn();
              }}
            >
              <label className="block text-sm">
                Username
                <input
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                />
              </label>
              <label className="block text-sm">
                Password
                <input
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </label>
              {loginError && (
                <p role="alert" className="text-sm text-destructive">
                  {loginError}
                </p>
              )}
              <Button className="w-full" disabled={signing}>
                {signing ? "Signing in…" : "Sign in"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="w-full"
                onClick={toggleTheme}
              >
                {dark ? <Sun /> : <Moon />}Change appearance
              </Button>
            </form>
          </CardContent>
        </Card>
      </main>
    );
}

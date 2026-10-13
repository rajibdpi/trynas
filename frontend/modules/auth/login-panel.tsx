"use client";

import { useErrorNotification } from "@/modules/shared/notification-provider";

import { useEffect, useState } from "react";
import { api } from "@/modules/shared/api";
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
  useErrorNotification(loginError);
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [requesting, setRequesting] = useState(false);
  const [registrationError, setRegistrationError] = useState("");
  useErrorNotification(registrationError);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void api("/api/auth/registration", { signal: controller.signal }).then(data => setRegistrationEnabled(data.enabled)).catch(() => {});
    return () => controller.abort();
  }, []);
  const register = async () => {
    setRequesting(true); setRegistrationError(""); setNotice("");
    try {
      await api("/api/auth/register", { method: "POST", body: JSON.stringify({ username, password }) });
      setPassword(""); setRegistering(false);
      setNotice("Account requested. Wait for administrator approval before signing in.");
    } catch (error) { setRegistrationError(error instanceof Error ? error.message : "Could not request account"); }
    finally { setRequesting(false); }
  };
  return (
      <main className="login-screen grid min-h-0 flex-1 place-items-center overflow-y-auto p-4">
        <Card className="relative w-full max-w-md">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="absolute right-4 top-4 text-muted-foreground"
            aria-label="Change appearance"
            title="Change appearance"
            onClick={toggleTheme}
          >
            {dark ? <Sun aria-hidden="true" /> : <Moon aria-hidden="true" />}
          </Button>
          <CardHeader>
            <div className="flex items-center gap-2 pr-10 text-xl font-bold text-primary">
              <HardDrive />
              TryNAS <span className="text-xs">V5</span>
            </div>
            <CardTitle className="mt-4 text-2xl">{registering ? "Request an account" : "Welcome back"}</CardTitle>
            <p className="text-sm text-muted-foreground">
              {registering ? "An administrator must approve your account and create your folder." : "Sign in to your private NAS."}
            </p>
          </CardHeader>
          <CardContent>
            <form
              className="space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                void (registering ? register() : signIn());
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
                  autoComplete={registering ? "new-password" : "current-password"}
                  minLength={registering ? 8 : undefined}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </label>

              <Button className="w-full" disabled={signing || requesting}>
                {requesting ? "Requesting…" : registering ? "Request account" : signing ? "Signing in…" : "Sign in"}
              </Button>
              {notice && <p role="status" className="text-sm">{notice}</p>}
              {registrationEnabled && <Button type="button" variant="outline" className="w-full" disabled={signing || requesting} onClick={() => { setRegistering(!registering); setRegistrationError(""); setNotice(""); setPassword(""); }}>{registering ? "Back to sign in" : "Request an account"}</Button>}
            </form>
          </CardContent>
        </Card>
      </main>
    );
}

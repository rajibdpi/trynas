"use client";

import { Plus } from "lucide-react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { type User, type Command } from "@/modules/shared/types";
import { fmt } from "@/modules/shared/format";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText } from "@/modules/shared/feedback";

export function UserPanel({
  me,
  ask,
  notify,
  onChanged,
}: {
  me: User;
  ask: (c: Command) => void;
  notify: (s: string) => void;
  onChanged: () => void;
}) {
  const users = useResource<User[]>("/api/users");
  const edit = (user?: User) =>
    ask({
      title: user ? "Edit user" : "Create user",
      description:
        "Quota 0 means unlimited. New users get a home folder named after their username with write access. Password changes revoke their sessions.",
      fields: [
        {
          key: "username",
          label: "Username",
          value: user?.username || "",
          required: true,
        },
        {
          key: "password",
          label: user
            ? "New password (blank to keep)"
            : "Password (8–72 bytes)",
          type: "password",
          value: "",
          required: !user,
        },
        {
          key: "role",
          label: "Role",
          value: user?.role || "user",
          options: [
            { value: "user", label: "User" },
            { value: "admin", label: "Administrator" },
          ],
        },
        {
          key: "quota",
          label: "Quota in GiB (0 = unlimited)",
          type: "number",
          value: String((user?.quota || 0) / 1073741824),
          required: true,
        },
      ],
      submit: "Save user",
      run: async (v) => {
        const quota = Math.round(Number(v.quota) * 1073741824);
        if (!Number.isSafeInteger(quota) || quota < 0)
          throw new Error("Enter a valid quota");
        await api("/api/users", {
          method: "POST",
          body: JSON.stringify({
            id: user?.id || 0,
            username: v.username,
            password: v.password,
            role: v.role,
            quota,
          }),
        });
        await users.reload();
        notify("User saved");
        onChanged();
      },
    });
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Users & quotas</CardTitle>
          <Button onClick={() => edit()}>
            <Plus />
            User
          </Button>
        </div>
        <p className="text-sm text-muted-foreground">
          Uploaded, copied and ZIP files count toward quota, including trash.
          Pre-existing files remain unassigned.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={users.error} />
        {users.data?.map((user) => (
          <div
            key={user.id}
            className="flex flex-wrap items-center gap-3 border-b py-4"
          >
            <div className="flex-1">
              <p className="font-semibold">
                {user.username}
                {user.id === me.id ? " (you)" : ""}
              </p>
              <p className="text-xs text-muted-foreground">
                {user.role} · {fmt(user.used)} /{" "}
                {user.quota ? fmt(user.quota) : "Unlimited"}
              </p>
              {!!user.quota && (
                <div className="mt-2 max-w-xs">
                  <Progress value={((user.used || 0) / user.quota) * 100} />
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={() => edit(user)}>
              Edit
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={user.id === me.id}
              onClick={() =>
                ask({
                  title: `Delete ${user.username}?`,
                  description:
                    "Remove owned files and trash first. This removes sessions, shares, favorites and permissions.",
                  fields: [],
                  danger: true,
                  submit: "Delete user",
                  run: async () => {
                    await api(`/api/users/${user.id}`, { method: "DELETE" });
                    await users.reload();
                    notify("User deleted");
                  },
                })
              }
            >
              Delete
            </Button>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

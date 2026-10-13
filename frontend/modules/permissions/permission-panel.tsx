"use client";

import { Shield } from "lucide-react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { type User, type Command } from "@/modules/shared/types";

import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

export function PermissionPanel({
  ask,
  notify,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
}) {
  const rules = useResource<any[]>("/api/permissions"),
    users = useResource<User[]>("/api/users");
  const edit = (rule?: any) => {
    const options = (users.data || [])
      .filter((u) => u.role !== "admin")
      .map((u) => ({ value: String(u.id), label: u.username }));
    if (!options.length) {
      notify("Create a standard user first. Administrators have full access.");
      return;
    }
    ask({
      title: rule ? "Edit permission" : "Grant folder access",
      description:
        "Enter an existing relative folder; blank means root. The nearest rule applies to descendants. Deny overrides an inherited grant.",
      fields: [
        {
          key: "userId",
          label: "User",
          value: String(rule?.userId || options[0].value),
          options,
          disabled: !!rule,
        },
        {
          key: "path",
          label: "Folder (blank = root)",
          value: rule?.path || "",
          disabled: !!rule,
        },
        {
          key: "access",
          label: "Access",
          value: rule?.access || "read",
          options: [
            { value: "read", label: "Read only" },
            { value: "write", label: "Read & write" },
            { value: "none", label: "Deny" },
          ],
        },
      ],
      submit: "Save permission",
      run: async (v) => {
        await api("/api/permissions", {
          method: "POST",
          body: JSON.stringify({
            userId: Number(v.userId),
            path: v.path,
            access: v.access,
          }),
        });
        await rules.reload();
        notify("Permission saved");
      },
    });
  };
  return (
    <Card>
      <CardHeader>
        <div className="flex justify-between gap-2">
          <CardTitle>Folder permissions</CardTitle>
          <Button disabled={!users.data} onClick={() => edit()}>
            <Shield />
            Grant access
          </Button>
        </div>
        <p className="text-sm text-muted-foreground">
          Normal users can access only their assigned home folder and its contents. Permissions apply
          within that home folder. Recursive operations respect nested rules.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={rules.error || users.error} />
        {rules.data?.map((rule) => (
          <div
            key={`${rule.userId}:${rule.path}`}
            className="flex flex-wrap items-center gap-3 border-b py-4"
          >
            <div className="flex-1">
              <b>{rule.username}</b>
              <p className="text-sm text-muted-foreground">
                /{rule.path} · {rule.access}
              </p>
            </div>
            <Button size="sm" variant="outline" onClick={() => edit(rule)}>
              Edit
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() =>
                ask({
                  title: "Remove rule?",
                  description:
                    "Access follows the nearest parent rule. Without a parent rule, access is denied.",
                  fields: [],
                  submit: "Remove rule",
                  run: async () => {
                    await api("/api/permissions", {
                      method: "POST",
                      body: JSON.stringify({
                        userId: rule.userId,
                        path: rule.path,
                        access: "inherit",
                      }),
                    });
                    await rules.reload();
                  },
                })
              }
            >
              Remove
            </Button>
          </div>
        ))}
        {rules.data?.length === 0 && <Empty text="No folder grants yet." />}
      </CardContent>
    </Card>
  );
}

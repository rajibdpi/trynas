"use client";

import { api } from "@/modules/shared/api";
import { useResource } from "@/modules/shared/use-resource";
import { type Command } from "@/modules/shared/types";
import { Button } from "@/components/ui/button";
import { ErrorText } from "@/modules/shared/feedback";

type Request = { id: number; username: string; createdAt: string };

export function RegistrationRequests({ ask, notify, onChanged }: {
  ask: (command: Command) => void;
  notify: (text: string) => void;
  onChanged: () => void;
}) {
  const requests = useResource<Request[]>("/api/users/requests", 10000);
  return <section className="mb-6 space-y-3" aria-label="Pending account requests">
    <h3 className="font-semibold">Pending account requests</h3>
    <ErrorText text={requests.error} />
    {requests.data?.length === 0 && <p className="text-sm text-muted-foreground">No pending requests.</p>}
    {requests.data?.map(request => <div key={request.id} className="flex flex-wrap items-center gap-3 rounded-lg border p-3">
      <span className="flex-1 font-medium">{request.username}</span>
      <Button size="sm" onClick={() => ask({
        title: `Approve ${request.username}?`,
        description: "Create this account and its private folder with write access. Quota 0 means unlimited.",
        fields: [{ key: "quota", label: "Quota in GiB (0 = unlimited)", type: "number", value: "0", required: true }],
        submit: "Approve account",
        run: async values => {
          const quota = Math.round(Number(values.quota) * 1073741824);
          if (!Number.isSafeInteger(quota) || quota < 0) throw new Error("Enter a valid quota");
          await api(`/api/users/requests/${request.id}/approve`, { method: "POST", body: JSON.stringify({ quota }) });
          await requests.reload(); onChanged(); notify("Account approved and folder created");
        },
      })}>Approve</Button>
      <Button variant="outline" size="sm" onClick={() => ask({
        title: `Reject ${request.username}?`, description: "Remove this account request without creating an account or folder.",
        fields: [], danger: true, submit: "Reject request",
        run: async () => { await api(`/api/users/requests/${request.id}`, { method: "DELETE" }); await requests.reload(); notify("Request rejected"); },
      })}>Reject</Button>
    </div>)}
  </section>;
}

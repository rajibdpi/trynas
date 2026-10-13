"use client";

import { useErrorNotification } from "@/modules/shared/notification-provider";

import { Select } from "@/components/ui/select";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { FolderPicker } from "@/modules/files/folder-picker";
import { type Command } from "@/modules/shared/types";
import { message } from "@/modules/shared/format";
import { ErrorText } from "@/modules/shared/feedback";

export function CommandDialog({
  command,
  close,
}: {
  command: Command | null;
  close: () => void;
}) {
  const [values, setValues] = useState<Record<string, string>>({}),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [destinationValid, setDestinationValid] = useState(false);
  useEffect(() => {
    setValues(
      Object.fromEntries(command?.fields.map((f) => [f.key, f.value]) || []),
    );
    setError("");
    setDestinationValid(false);
  }, [command]);
  useErrorNotification(error);
  return (
    <Dialog
      open={!!command}
      onOpenChange={(open) => {
        if (!open && !busy) close();
      }}
    >
      <DialogContent
        onEscapeKeyDown={(e) => {
          if (busy) e.preventDefault();
        }}
        onPointerDownOutside={(e) => {
          if (busy) e.preventDefault();
        }}
      >
        <DialogTitle className="pr-6 text-xl font-semibold">
          {command?.title}
        </DialogTitle>
        <DialogDescription className="mt-2 text-sm text-muted-foreground">
          {command?.description}
        </DialogDescription>
        <form
          className="mt-5 space-y-4"
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              !command ||
              busy ||
              (command.destinationFolders && !destinationValid)
            )
              return;
            setBusy(true);
            setError("");
            try {
              await command.run(values);
              close();
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          {command?.destinationFolders && (
            <FolderPicker
              key={command.title}
              value={values.destination ?? command.fields[0].value}
              excluded={command.destinationFolders}
              basePath={command.destinationRoot || ""}
              disabled={busy}
              onChange={(destination) => {
                setDestinationValid(false);
                setValues((old) => ({ ...old, destination }));
              }}
              onValidity={setDestinationValid}
            />
          )}
          {command?.fields
            .filter(
              (f) => !(command.destinationFolders && f.key === "destination") &&
                (!f.visibleWhen || (values[f.visibleWhen.key] ?? command.fields.find(field => field.key === f.visibleWhen!.key)?.value) === f.visibleWhen.value),
            )
            .map((f) => (
              <label key={f.key} className="block text-sm font-medium">
                <span className="mb-1 block">{f.label}</span>
                {f.options ? (
                  <Select
                    disabled={f.disabled}
                    value={values[f.key] ?? f.value}
                    onValueChange={(value) =>
                      setValues({ ...values, [f.key]: value })
                    }
                  >
                    {f.options.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </Select>
                ) : (
                  <input
                    disabled={f.disabled}
                    type={f.type || "text"}
                    value={values[f.key] ?? f.value}
                    onChange={(e) =>
                      setValues({ ...values, [f.key]: e.target.value })
                    }
                    required={f.required}
                    min={f.type === "number" ? 0 : undefined}
                    step={f.type === "number" ? "any" : undefined}
                    autoComplete={
                      f.type === "password" ? "new-password" : "off"
                    }
                  />
                )}
              </label>
            ))}
          <ErrorText text={error} />
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={close}
            >
              Cancel
            </Button>
            <Button
              variant={command?.danger ? "destructive" : "default"}
              disabled={
                busy || (!!command?.destinationFolders && !destinationValid)
              }
            >
              {busy ? "Working…" : command?.submit}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

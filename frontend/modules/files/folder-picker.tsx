"use client";

import { useEffect, useState } from "react";
import { ChevronRight, Folder } from "lucide-react";
import { api } from "@/modules/shared/api";
import { cn } from "@/modules/shared/utils";

type Listing = {
  writable: boolean;
  items: { name: string; path: string; dir: boolean; writable: boolean }[];
};
type PickerProps = {
  basePath?: string;
  value: string;
  excluded: string[];
  disabled?: boolean;
  onChange: (path: string) => void;
  onValidity: (valid: boolean) => void;
};
const within = (path: string, base: string) =>
  path === base || path.startsWith(base + "/");

export function FolderPicker(props: PickerProps) {
  const [initialPath] = useState(props.value);
  const [error, setError] = useState("");
  const [validationAttempt, setValidationAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    props.onValidity(false);
    setError("");
    if (props.excluded.some((p) => within(props.value, p))) {
      setError("Choose a folder outside the selected source folders.");
      return () => controller.abort();
    }
    void api("/api/files?path=" + encodeURIComponent(props.value), {
      signal: controller.signal,
    })
      .then((listing: Listing) => {
        if (controller.signal.aborted) return;
        props.onValidity(listing.writable);
        if (!listing.writable) setError("Choose a folder with write access.");
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted)
          setError(
            e instanceof Error ? e.message.trim() : "Cannot read this folder.",
          );
      });
    return () => controller.abort();
  }, [props.value, props.excluded, props.onValidity, validationAttempt]);

  return (
    <div className="space-y-3">
      <p className="text-sm font-medium" id="destination-tree-label">
        Select destination folder
      </p>
      <div
        className="max-h-72 overflow-auto rounded-lg border bg-background p-2"
        aria-labelledby="destination-tree-label"
      >
        <ul>
          <FolderBranch
            {...props}
            path={props.basePath || ""}
            name={props.basePath ? "Home" : "Storage root"}
            initialPath={initialPath}
            writable={true}
          />
        </ul>
      </div>
      <p className="break-all rounded-lg bg-accent p-3 text-sm">
        Selected path: <strong>/{props.value}</strong>
      </p>
      {error && (
        <div>
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
          <button
            type="button"
            disabled={props.disabled}
            className="text-xs text-primary"
            onClick={() => setValidationAttempt((n) => n + 1)}
          >
            Check again
          </button>
        </div>
      )}
    </div>
  );
}

function FolderBranch(
  props: PickerProps & {
    path: string;
    name: string;
    initialPath: string;
    writable: boolean;
  },
) {
  const [expanded, setExpanded] = useState(
    props.path === "" || within(props.initialPath, props.path),
  );
  const [listing, setListing] = useState<Listing | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const blocked = props.excluded.some((p) => within(props.path, p));
  useEffect(() => {
    if (!expanded || blocked) return;
    const controller = new AbortController();
    setLoading(true);
    setError("");
    void api("/api/files?path=" + encodeURIComponent(props.path), {
      signal: controller.signal,
    })
      .then((result: Listing) => {
        if (!controller.signal.aborted) setListing(result);
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted)
          setError(
            e instanceof Error ? e.message.trim() : "Unable to load folders",
          );
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [expanded, blocked, props.path, attempt]);
  const writable =
    listing?.writable ?? (props.path === "" ? false : props.writable);
  return (
    <li>
      <div
        className={cn(
          "flex items-center gap-2 rounded p-1.5",
          props.value === props.path && "bg-primary/10",
          blocked && "opacity-50",
        )}
      >
        <button
          type="button"
          disabled={blocked || props.disabled}
          className="rounded p-1 hover:bg-accent"
          aria-label={`${expanded ? "Collapse" : "Expand"} ${props.name}`}
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
        >
          <ChevronRight
            className={cn(
              "size-4 transition-transform",
              expanded && "rotate-90",
            )}
          />
        </button>
        <label
          className={cn(
            "flex min-w-0 flex-1 items-center gap-2 text-sm",
            (!writable || blocked) && "text-muted-foreground",
          )}
        >
          <input
            type="radio"
            name="destination-folder"
            value={props.path}
            checked={props.value === props.path}
            disabled={!writable || blocked || props.disabled}
            onChange={() => props.onChange(props.path)}
            aria-label={`Select ${props.path ? "/" + props.path : "storage root"}`}
          />
          <Folder className="size-4 shrink-0 text-amber-500" />
          <span className="break-all">{props.name}</span>
          {!writable && !blocked && (
            <span className="ml-auto shrink-0 text-xs">Read only</span>
          )}
          {blocked && <span className="ml-auto text-xs">Source</span>}
        </label>
      </div>
      {expanded && !blocked && (
        <div className="ml-5 border-l pl-2">
          {loading && (
            <p className="p-2 text-xs text-muted-foreground">
              Loading folders…
            </p>
          )}
          {error && (
            <div className="p-2">
              <p role="alert" className="text-xs text-destructive">
                {error}
              </p>
              <button
                type="button"
                disabled={props.disabled}
                className="mt-1 text-xs text-primary"
                onClick={() => setAttempt((n) => n + 1)}
              >
                Retry
              </button>
            </div>
          )}
          {!loading && !error && listing && (
            <ul>
              {listing.items
                .filter((item) => item.dir)
                .map((item) => (
                  <FolderBranch
                    {...props}
                    key={item.path}
                    path={item.path}
                    name={item.name}
                    writable={item.writable}
                  />
                ))}
            </ul>
          )}
          {!loading &&
            !error &&
            listing &&
            !listing.items.some((item) => item.dir) && (
              <p className="p-2 text-xs text-muted-foreground">No subfolders</p>
            )}
        </div>
      )}
    </li>
  );
}

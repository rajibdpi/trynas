export const ERROR_NOTIFICATION_EVENT = "trynas:error-notification";

export function errorNotificationText(error: unknown): string {
  if (error instanceof Error && error.name === "AbortError") return "";
  const text = typeof error === "string" ? error.trim() : error instanceof Error ? error.message.trim() : "Something went wrong";
  return text === "Upload cancelled" ? "" : text;
}

export function reportError(error: unknown) {
  const text = errorNotificationText(error);
  if (text && typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(ERROR_NOTIFICATION_EVENT, { detail: text }));
  }
}

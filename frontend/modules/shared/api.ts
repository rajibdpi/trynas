import { reportError } from "./error-notifications";

// Development requests use the Next.js proxy so LAN clients never call their own localhost.
export const API = process.env.NEXT_PUBLIC_EMBEDDED === "true" || process.env.NODE_ENV === "development" ? "" : (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080");
export function shareURL(token: string) { return `${API || (typeof window !== "undefined" ? window.location.origin : "")}/s/${token}`; }
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}
export async function api(path: string, opts: RequestInit & { quietUnauthorized?: boolean } = {}) {
  const { quietUnauthorized = false, ...request } = opts;
  try {
    const r = await fetch(API + path, {
      ...request,
      credentials: "include",
      headers: {
        ...(request.body instanceof FormData ? {} : { "Content-Type": "application/json" }),
        ...(request.headers || {}),
      },
    });
    if (!r.ok) {
      const text = await r.text();
      const expired = r.status === 401 && path !== "/api/auth/login";
      if (expired && !quietUnauthorized && typeof window !== "undefined") {
        window.dispatchEvent(new Event("trynas:unauthorized"));
      }
      throw new ApiError(r.status, expired && !quietUnauthorized ? "Your session expired. Please sign in again." : text);
    }
    return await r.json();
  } catch (error) {
    if (!(quietUnauthorized && error instanceof ApiError && error.status === 401)) reportError(error);
    throw error;
  }
}

export function uploadFiles(
  data: FormData,
  onProgress: (loaded: number, total: number) => void,
  signal?: AbortSignal,
): Promise<{ bytes: number; files: number }> {
  return new Promise<{ bytes: number; files: number }>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", API + "/api/files/upload");
    xhr.withCredentials = true;
    const abort = () => xhr.abort();
    signal?.addEventListener("abort", abort, { once: true });
    const cleanup = () => signal?.removeEventListener("abort", abort);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded, e.total);
    };
    xhr.onload = () => {
      cleanup();
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText));
        } catch {
          reject(new Error("Invalid upload response"));
        }
      } else
        reject(
          new ApiError(xhr.status, xhr.responseText.trim() || "Upload failed"),
        );
    };
    xhr.onerror = () => {
      cleanup();
      reject(new Error("Cannot reach the upload server"));
    };
    xhr.onabort = () => {
      cleanup();
      reject(Object.assign(new Error("Upload cancelled"), { name: "AbortError" }));
    };
    if (signal?.aborted) {
      cleanup();
      reject(Object.assign(new Error("Upload cancelled"), { name: "AbortError" }));
      return;
    }
    xhr.send(data);
  }).catch(error => {
    if (error instanceof ApiError && error.status === 401) {
      error.message = "Your session expired. Please sign in again.";
      window.dispatchEvent(new Event("trynas:unauthorized"));
    }
    reportError(error);
    throw error;
  });
}

export const API = process.env.NEXT_PUBLIC_EMBEDDED === "true" ? "" : (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080");
export function shareURL(token:string){return `${API || (typeof window !== "undefined" ? window.location.origin : "")}/s/${token}`;}
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}
export async function api(path: string, opts: RequestInit = {}) {
  const r = await fetch(API + path, {
    ...opts,
    credentials: "include",
    headers: {
      ...(opts.body instanceof FormData
        ? {}
        : { "Content-Type": "application/json" }),
      ...(opts.headers || {}),
    },
  });
  if (!r.ok) {
    if (
      r.status === 401 &&
      typeof window !== "undefined" &&
      path !== "/api/auth/login"
    )
      window.dispatchEvent(new Event("trynas:unauthorized"));
    throw new ApiError(r.status, await r.text());
  }
  return r.json();
}

export function uploadFiles(
  data: FormData,
  onProgress: (loaded: number, total: number) => void,
  signal?: AbortSignal,
): Promise<{ bytes: number; files: number }> {
  return new Promise((resolve, reject) => {
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
      reject(new Error("Upload cancelled"));
    };
    if (signal?.aborted) {
      cleanup();
      reject(new Error("Upload cancelled"));
      return;
    }
    xhr.send(data);
  });
}

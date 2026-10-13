import { readFileSync } from "node:fs";
import { resolve } from "node:path";

export function proxyUploadLimitBytes(projectRoot, environment = process.env) {
  let value = environment.PROXY_UPLOAD_LIMIT_MB;
  if (!value) {
    try {
      const contents = readFileSync(resolve(projectRoot, environment.TRYNAS_CONFIG_FILE || ".env"), "utf8");
      // Read only this numeric setting; never import server secrets into Next.js.
      const match = contents.match(/^\s*(?:export\s+)?PROXY_UPLOAD_LIMIT_MB\s*=\s*(?:"(\d+)"|'(\d+)'|(\d+))\s*(?:#.*)?$/m);
      value = match?.[1] || match?.[2] || match?.[3];
      if (!value && /^\s*(?:export\s+)?PROXY_UPLOAD_LIMIT_MB\s*=/m.test(contents)) {
        throw new Error("PROXY_UPLOAD_LIMIT_MB must be an integer from 1 to 10240");
      }
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  if (value && !/^\d+$/.test(value)) {
    throw new Error("PROXY_UPLOAD_LIMIT_MB must be an integer from 1 to 10240");
  }
  const limit = value ? Number(value) : 10240;
  if (!Number.isInteger(limit) || limit < 1 || limit > 10240) {
    throw new Error("PROXY_UPLOAD_LIMIT_MB must be an integer from 1 to 10240");
  }
  return limit * 1024 ** 2;
}

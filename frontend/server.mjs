import { createServer } from "node:http";
import { parseArgs } from "node:util";
import { fileURLToPath } from "node:url";
import next from "next";
import { createUploadProxy, isUploadRequest } from "./upload-proxy.mjs";
import { proxyUploadLimitBytes } from "./upload-limit.mjs";

const { values } = parseArgs({ options: {
  dev: { type: "boolean", default: false },
  port: { type: "string", short: "p", default: process.env.PORT || "3000" },
  hostname: { type: "string", short: "H", default: "0.0.0.0" },
} });
process.env.NODE_ENV ||= values.dev ? "development" : "production";
const port = Number(values.port);
const server = createServer((req, res) => {
  if (isUploadRequest(req)) {
    // Node's default requestTimeout also imposes a total body-read deadline.
    req.setTimeout(0);
    res.setTimeout(0);
    upload(req, res);
  } else {
    handle(req, res);
  }
});
server.requestTimeout = 0;
server.headersTimeout = 10_000;
server.keepAliveTimeout = 5_000;
const app = next({ dev: values.dev, hostname: values.hostname, port, httpServer: server });
const handle = app.getRequestHandler();
await app.prepare();
const upload = createUploadProxy(process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080",
  proxyUploadLimitBytes(fileURLToPath(new URL("../", import.meta.url))));
server.on("upgrade", app.getUpgradeHandler());
server.listen(port, values.hostname, () => console.log(`TryNAS frontend listening on http://${values.hostname}:${port}`));
for (const signal of ["SIGINT", "SIGTERM"]) process.on(signal, async () => {
  server.closeAllConnections();
  server.close();
  await app.close();
  process.exit(0);
});

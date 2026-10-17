import http from "node:http";
import https from "node:https";
import { Transform } from "node:stream";

export function isUploadRequest(request) {
  const path = request.url?.split("?", 1)[0];
  return request.method === "POST" && (path === "/api/files/upload" || path === "/api/storage/upload");
}

// Uploads bypass Next's rewrite timeout and stream with backpressure, without buffering.
export function createUploadProxy(target, limit) {
  const backend = new URL(target);
  return (req, res) => {
    const fail = (status, message) => {
      if (res.destroyed || res.writableEnded) return;
      if (res.headersSent) return res.destroy();
      res.writeHead(status, { "Content-Type": "text/plain", "Connection": "close" });
      res.end(message);
    };
    if (Number(req.headers["content-length"]) > limit) {
      fail(413, "Upload exceeds the configured proxy limit");
      return;
    }
    const headers = { ...req.headers, host: backend.host, "x-forwarded-host": req.headers.host,
      "x-forwarded-proto": req.socket.encrypted ? "https" : "http", "x-forwarded-for": req.socket.remoteAddress };
    // Never pass a client's hop-by-hop headers to the upstream connection.
    for (const name of String(req.headers.connection || "").split(",")) delete headers[name.trim().toLowerCase()];
    for (const name of ["connection", "keep-alive", "proxy-connection", "transfer-encoding", "upgrade", "te", "trailer", "expect"]) delete headers[name];
    const transport = backend.protocol === "https:" ? https : http;
    const upstream = transport.request(new URL(req.url, backend), { method: req.method, headers, agent: false });
    upstream.setTimeout(0);
    let bytes = 0;
    const body = new Transform({
      transform(chunk, encoding, done) {
        bytes += chunk.length;
        if (bytes > limit) {
          fail(413, "Upload exceeds the configured proxy limit");
          upstream.destroy();
          done(new Error("Upload exceeds proxy limit"));
        } else done(null, chunk);
      },
    });
    body.on("error", () => req.unpipe(body));
    upstream.on("response", response => {
      res.writeHead(response.statusCode || 502, response.headers);
      response.on("error", () => res.destroy());
      response.pipe(res);
    });
    upstream.on("error", () => {
      req.unpipe(body);
      body.destroy();
      fail(502, "Cannot reach the upload server");
    });
    const cancel = () => { req.unpipe(body); body.destroy(); upstream.destroy(); };
    req.on("aborted", cancel);
    res.on("close", cancel);
    req.on("error", cancel);
    req.pipe(body).pipe(upstream);
  };
}

import { test } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import { once } from "node:events";
import { Readable } from "node:stream";
import { setTimeout as delay } from "node:timers/promises";
import { createUploadProxy, isUploadRequest } from "../upload-proxy.mjs";

test("LAN proxy waits over 30 seconds for a multipart upload over 100 MiB to finish processing", { timeout: 60_000 }, async () => {
  const chunk = Buffer.alloc(1024 ** 2, 120);
  const preamble = Buffer.from('--upload-test\r\nContent-Disposition: form-data; name="files"; filename="large.bin"\r\nContent-Type: application/octet-stream\r\n\r\n');
  const ending = Buffer.from("\r\n--upload-test--\r\n");
  const expected = preamble.length + 101 * chunk.length + ending.length;
  const backend = http.createServer(async (req, res) => {
    let received = 0;
    for await (const bytes of req) received += bytes.length;
    // Large files can take time to store after the browser finishes sending.
    await delay(31_000);
    res.setHeader("Content-Type", "application/json");
    res.end(JSON.stringify({ received, contentType: req.headers["content-type"] }));
  });
  const proxy = http.createServer((req, res) => {
    createUploadProxy(`http://127.0.0.1:${backend.address().port}`, 10 * 1024 ** 3)(req, res);
  });
  try {
    backend.listen(0, "127.0.0.1");
    await once(backend, "listening");
    proxy.listen(0, "127.0.0.1");
    await once(proxy, "listening");
    async function* body() {
      yield preamble;
      for (let i = 0; i < 101; i++) {
        yield chunk;
      }
      yield ending;
    }
    const started = Date.now();
    const response = await fetch(`http://127.0.0.1:${proxy.address().port}/api/files/upload`, {
      method: "POST", duplex: "half", body: Readable.from(body()),
      headers: { "Content-Type": "multipart/form-data; boundary=upload-test", "Content-Length": String(expected) },
      signal: AbortSignal.timeout(50_000),
    });
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { received: expected, contentType: "multipart/form-data; boundary=upload-test" });
    assert.ok(Date.now() - started > 30_000);
  } finally {
    proxy.closeAllConnections();
    backend.closeAllConnections();
    await Promise.all([new Promise(resolve => proxy.close(resolve)), new Promise(resolve => backend.close(resolve))]);
  }
});

test("only file upload POST requests bypass ordinary proxy deadlines", () => {
  assert.equal(isUploadRequest({ method: "POST", url: "/api/files/upload" }), true);
  assert.equal(isUploadRequest({ method: "POST", url: "/api/storage/upload?storage=remote" }), true);
  assert.equal(isUploadRequest({ method: "POST", url: "/api/files/upload/check" }), false);
  assert.equal(isUploadRequest({ method: "GET", url: "/api/files/upload" }), false);
});

test("streaming uploads still enforce proxy size limits with and without Content-Length", async () => {
  const backend = http.createServer((req, res) => { req.resume(); req.on("end", () => res.end("ok")); });
  let proxy;
  try {
    backend.listen(0, "127.0.0.1");
    await once(backend, "listening");
    proxy = http.createServer(createUploadProxy(`http://127.0.0.1:${backend.address().port}`, 16));
    proxy.listen(0, "127.0.0.1");
    await once(proxy, "listening");
    const url = `http://127.0.0.1:${proxy.address().port}/api/files/upload`;
    for (const body of ["x".repeat(32), Readable.from([Buffer.alloc(32)])]) {
      const response = await fetch(url, { method: "POST", body, duplex: "half" });
      assert.equal(response.status, 413);
      await response.text();
    }
  } finally {
    proxy?.closeAllConnections();
    backend.closeAllConnections();
    await Promise.all([new Promise(resolve => proxy ? proxy.close(resolve) : resolve()), new Promise(resolve => backend.close(resolve))]);
  }
});

test("cancelling an upload closes the upstream transfer", { timeout: 5_000 }, async () => {
  let started, cancelled;
  const received = new Promise(resolve => { started = resolve; });
  const aborted = new Promise(resolve => { cancelled = resolve; });
  const backend = http.createServer(req => {
    req.on("aborted", cancelled);
    req.on("error", () => {});
    req.on("data", started);
  });
  let proxy, client;
  try {
    backend.listen(0, "127.0.0.1");
    await once(backend, "listening");
    proxy = http.createServer(createUploadProxy(`http://127.0.0.1:${backend.address().port}`, 1024));
    proxy.listen(0, "127.0.0.1");
    await once(proxy, "listening");
    client = http.request(`http://127.0.0.1:${proxy.address().port}/api/files/upload`, { method: "POST" });
    client.on("error", () => {});
    client.write("partial file");
    await received;
    client.destroy();
    await aborted;
  } finally {
    client?.destroy();
    proxy?.closeAllConnections();
    backend.closeAllConnections();
    await Promise.all([new Promise(resolve => proxy ? proxy.close(resolve) : resolve()), new Promise(resolve => backend.close(resolve))]);
  }
});

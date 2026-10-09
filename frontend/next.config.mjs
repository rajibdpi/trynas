import { networkInterfaces } from "node:os";

const embedded = process.env.TRYNAS_EMBEDDED_BUILD === "true";
/** @type {import('next').NextConfig} */
const config = {
  skipTrailingSlashRedirect: true,
  allowedDevOrigins: [...new Set(Object.values(networkInterfaces()).flat().filter(address => address && !address.internal).map(address => address.address))],
  distDir: embedded ? "dist" : process.env.NODE_ENV === "development" ? ".next-dev" : ".next",
  ...(!embedded ? {
    async rewrites() {
      return [
        ...(process.env.NODE_ENV === "development" ? [{ source: "/api/:path*", destination: `${process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080"}/api/:path*` }, { source: "/s/:path*", destination: `${process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080"}/s/:path*` }] : []),
        { source: "/files/:path*", destination: "/files" }
      ];
    }
  } : {}),
  ...(embedded ? { output: "export", images: { unoptimized: true } } : {}),
};
export default config;

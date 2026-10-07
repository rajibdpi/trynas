const embedded = process.env.TRYNAS_EMBEDDED_BUILD === "true";
/** @type {import('next').NextConfig} */
const config = {
  distDir: process.env.NODE_ENV === "development" ? ".next-dev" : ".next",
  ...(embedded ? { output: "export", images: { unoptimized: true } } : {}),
};
export default config;

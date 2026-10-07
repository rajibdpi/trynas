const embedded = process.env.TRYNAS_EMBEDDED_BUILD === "true";
/** @type {import('next').NextConfig} */
const config = {
  ...(embedded ? { output: "export", images: { unoptimized: true } } : {}),
};
export default config;

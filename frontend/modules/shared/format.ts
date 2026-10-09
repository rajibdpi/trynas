

export const fmt = (value: number = 0) => {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  while (value >= 1024 && i < 4) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(i ? 1 : 0)} ${units[i]}`;
};

export const message = (e: unknown) =>
  e instanceof Error ? e.message.trim() : "Something went wrong";

export const parent = (p: string) => p.split("/").slice(0, -1).join("/");

export const join = (a: string, b: string) => (a ? `${a}/${b}` : b);

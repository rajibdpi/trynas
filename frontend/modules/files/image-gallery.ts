import type { Item } from "@/modules/shared/types";

const imageExtensions = new Set(["jpg", "jpeg", "png", "gif", "webp", "svg", "avif", "bmp", "ico", "apng"]);
export function isImageFile(item: { name: string; dir: boolean }) {
  return !item.dir && imageExtensions.has(item.name.split(".").pop()?.toLowerCase() || "");
}
export function adjacentImage(images: Item[], currentPath: string, direction: -1 | 1): Item | undefined {
  const index = images.findIndex(image => image.path === currentPath);
  return index >= 0 && images.length > 1 ? images[(index + direction + images.length) % images.length] : undefined;
}

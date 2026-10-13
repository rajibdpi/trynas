import type { Item } from "@/modules/shared/types";

export const imageExtensions = ["jpg", "jpeg", "jpe", "jfif", "pjpeg", "pjp", "png", "apng", "gif", "webp", "svg", "svgz", "avif", "bmp", "dib", "ico", "cur", "tif", "tiff", "heic", "heif", "jxl", "raw", "arw", "cr2", "cr3", "nef", "nrw", "dng", "orf", "rw2", "pef", "raf", "srw", "psd", "psb", "tga", "dds"];
const imageExtensionSet = new Set(imageExtensions);
export function isImageFile(item: { name: string; dir: boolean }) {
  return !item.dir && imageExtensionSet.has(item.name.split(".").pop()?.toLowerCase() || "");
}
export function adjacentImage(images: Item[], currentPath: string, direction: -1 | 1): Item | undefined {
  const index = images.findIndex(image => image.path === currentPath);
  return index >= 0 && images.length > 1 ? images[(index + direction + images.length) % images.length] : undefined;
}

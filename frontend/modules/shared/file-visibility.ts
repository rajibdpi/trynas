export function isHiddenFile(item: { name: string; path: string }): boolean {
  return item.name.startsWith(".") || item.path.split("/").some(part => part.startsWith("."));
}

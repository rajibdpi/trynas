export function selectFiles(
  visible: string[],
  selected: string[],
  target: string,
  anchor: string | null,
  modifiers: { shift: boolean; additive: boolean },
): { selected: string[]; anchor: string } {
  const start = anchor === null ? -1 : visible.indexOf(anchor);
  const end = visible.indexOf(target);
  if (modifiers.shift && start >= 0 && end >= 0) {
    const range = visible.slice(Math.min(start, end), Math.max(start, end) + 1);
    return {
      selected: modifiers.additive ? [...new Set([...selected, ...range])] : range,
      anchor: anchor!,
    };
  }
  return {
    selected: modifiers.additive
      ? selected.includes(target) ? selected.filter(path => path !== target) : [...selected, target]
      : [target],
    anchor: target,
  };
}

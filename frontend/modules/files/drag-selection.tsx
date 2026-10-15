"use client";

import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type MouseEvent as ReactMouseEvent } from "react";

type Box = { left: number; top: number; right: number; bottom: number };
export function intersectsSelection(a: Box, b: Box) {
  return a.left <= b.right && a.right >= b.left && a.top <= b.bottom && a.bottom >= b.top;
}

export function useDragSelection({ selected, onSelect, scope, disabled = false }: { selected: string[]; onSelect: (paths: string[]) => void; scope: string; disabled?: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const cleanup = useRef<(() => void) | null>(null);
  const suppressClick = useRef(false);
  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [box, setBox] = useState<Box | null>(null);
  useEffect(() => {
    setBox(null);
    suppressClick.current = false;
    return () => { cleanup.current?.(); if (resetTimer.current) clearTimeout(resetTimer.current); };
  }, [scope]);
  const onPointerDownCapture = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (disabled || event.button !== 0 || event.pointerType !== "mouse" || event.detail > 1) return;
    const target = event.target as HTMLElement;
    if (target.closest("input, textarea, select, a, [contenteditable='true']") || (target.closest("button") && !target.closest("[data-file-select]"))) return;
    if (target.closest("[draggable='true']") && !event.ctrlKey && !event.metaKey && !event.shiftKey) return;
    const container = ref.current;
    if (!container) return;
    cleanup.current?.();
    if (resetTimer.current) clearTimeout(resetTimer.current);
    suppressClick.current = false;
    const origin = { x: event.clientX, y: event.clientY };
    const baseline = [...selected];
    const additive = event.ctrlKey || event.metaKey || event.shiftKey;
    let dragging = false;
    const move = (pointer: PointerEvent) => {
      if (pointer.pointerId !== event.pointerId) return;
      if (!dragging && Math.hypot(pointer.clientX - origin.x, pointer.clientY - origin.y) < 5) return;
      dragging = true;
      suppressClick.current = true;
      pointer.preventDefault();
      window.getSelection()?.removeAllRanges();
      const bounds = container.getBoundingClientRect();
      const next = {
        left: Math.max(bounds.left, Math.min(origin.x, pointer.clientX)),
        top: Math.max(bounds.top, Math.min(origin.y, pointer.clientY)),
        right: Math.min(bounds.right, Math.max(origin.x, pointer.clientX)),
        bottom: Math.min(bounds.bottom, Math.max(origin.y, pointer.clientY)),
      };
      setBox(next);
      const matches = [...container.querySelectorAll<HTMLElement>("[data-selection-id]")].filter(element => intersectsSelection(next, element.getBoundingClientRect())).map(element => element.dataset.selectionId!);
      onSelect(additive ? [...new Set([...baseline, ...matches])] : matches);
    };
    const clear = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", finish);
      window.removeEventListener("pointercancel", cancel);
      window.removeEventListener("keydown", key);
      window.removeEventListener("blur", cancel);
      cleanup.current = null;
    };
    const finish = (pointer: PointerEvent) => {
      if (pointer.pointerId !== event.pointerId) return;
      clear(); setBox(null);
      // The browser dispatches click immediately after pointerup.
      resetTimer.current = setTimeout(() => { suppressClick.current = false; }, 0);
    };
    const cancel = () => {
      if (dragging) onSelect(baseline);
      clear(); setBox(null);
      resetTimer.current = setTimeout(() => { suppressClick.current = false; }, 0);
    };
    const key = (keyboard: KeyboardEvent) => { if (keyboard.key === "Escape") { keyboard.preventDefault(); cancel(); } };
    cleanup.current = clear;
    window.addEventListener("pointermove", move, { passive: false });
    window.addEventListener("pointerup", finish);
    window.addEventListener("pointercancel", cancel);
    window.addEventListener("keydown", key);
    window.addEventListener("blur", cancel);
  };
  const onClickCapture = (event: ReactMouseEvent) => {
    if (suppressClick.current) { event.preventDefault(); event.stopPropagation(); }
  };
  const overlay = box && <div aria-hidden="true" className="pointer-events-none fixed z-40 rounded-sm border border-primary bg-primary/15" style={{ left: box.left, top: box.top, width: Math.max(0, box.right - box.left), height: Math.max(0, box.bottom - box.top) }} />;
  return { ref, onPointerDownCapture, onClickCapture, overlay };
}

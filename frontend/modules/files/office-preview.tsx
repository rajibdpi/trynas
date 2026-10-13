"use client";

import { useErrorNotification } from "@/modules/shared/notification-provider";

import { Select } from "@/components/ui/select";
import { useEffect, useState } from "react";
import { type WorkBook } from "xlsx";

export function OfficePreview({ url, ext, size }: { url: string; ext: string; size: number }) {
  const [workbook, setWorkbook] = useState<WorkBook | null>(null);
  const [sheet, setSheet] = useState("");
  const [rows, setRows] = useState<string[][]>([]);
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState("");
  useErrorNotification(error);
  useEffect(() => {
    const controller = new AbortController();
    if (ext === "doc") { setError("Legacy .doc files need conversion to .docx or PDF for preview. Download to open this file in Word or LibreOffice."); return; }
    if (size > 10 * 1024 * 1024) { setError("Office previews support files up to 10 MiB. Download to open the full document."); return; }
    void (async () => {
      try {
        const response = await fetch(url, { credentials: "include", signal: controller.signal });
        if (!response.ok) throw new Error(await response.text());
        const reader = response.body!.getReader();
        const chunks: Uint8Array[] = [];
        let length = 0;
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          length += value.length;
          if (length > 10 * 1024 * 1024) { await reader.cancel(); throw new Error("Document exceeds the 10 MiB preview limit."); }
          chunks.push(value);
        }
        const data = new Uint8Array(length);
        let offset = 0;
        for (const chunk of chunks) { data.set(chunk, offset); offset += chunk.length; }
        if (ext === "docx") {
          const mammoth = await import("mammoth/mammoth.browser");
          const result = await mammoth.extractRawText({ arrayBuffer: data.buffer });
          if (!controller.signal.aborted) setText(result.value);
        } else {
          const XLSX = await import("xlsx");
          const parsed = XLSX.read(data, { type: "array", sheetRows: 1000, cellHTML: false, cellFormula: false });
          if (!controller.signal.aborted) { setWorkbook(parsed); setSheet(parsed.SheetNames[0] || ""); }
        }
      } catch (error) { if (!controller.signal.aborted) setError(error instanceof Error ? error.message : "Could not read document"); }
    })();
    return () => controller.abort();
  }, [url, ext, size]);
  useEffect(() => {
    if (!workbook || !sheet) return;
    let active = true;
    void import("xlsx").then(XLSX => {
      const worksheet = workbook.Sheets[sheet];
      const range = XLSX.utils.decode_range(worksheet["!ref"] || "A1");
      range.e.r = Math.min(range.e.r, range.s.r + 999);
      range.e.c = Math.min(range.e.c, range.s.c + 99);
      const values = XLSX.utils.sheet_to_json<string[]>(worksheet, { header: 1, raw: false, defval: "", range });
      if (active) setRows(values);
    });
    return () => { active = false; };
  }, [workbook, sheet]);
  if (error) return <p className="text-sm text-muted-foreground">Download the file to open it.</p>;
  if (ext === "docx") return text === null ? <p>Loading Word document…</p> : <article className="max-h-[65vh] overflow-auto whitespace-pre-wrap rounded border p-5 text-sm">{text || "This document contains no text."}</article>;
  if (!workbook) return <p>Loading spreadsheet…</p>;
  return <div className="space-y-3"><label className="flex items-center gap-3 text-sm">Worksheet<Select className="!w-auto" value={sheet} onValueChange={value => setSheet(value)}>{workbook.SheetNames.map(name => <option key={name}>{name}</option>)}</Select></label><div className="max-h-[55vh] overflow-auto rounded border"><table className="w-full text-left text-sm"><tbody>{rows.map((row, index) => <tr key={index} className="border-b"><th className="bg-muted px-2 text-muted-foreground">{index + 1}</th>{row.map((cell, column) => <td key={column} className="whitespace-pre-wrap px-3 py-2">{String(cell)}</td>)}</tr>)}</tbody></table></div><p className="text-xs text-muted-foreground">Shows cell values from up to 1,000 rows and 100 columns per worksheet. Charts and macros are not displayed.</p></div>;
}

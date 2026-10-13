import { imageExtensions } from "./image-gallery";
import { File, FileImage, FileVideo, FileAudio, FileText, FileSpreadsheet, FileArchive, FileCode, FileSliders, FileCog, Presentation } from "lucide-react";

const types = [
  { extensions: imageExtensions, icon: FileImage, label: "Image", color: "text-pink-600 dark:text-pink-400", background: "bg-pink-500/10" },
  { extensions: ["mp4", "mkv", "mov", "avi", "webm", "m4v", "mpeg", "mpg", "wmv", "flv"], icon: FileVideo, label: "Video", color: "text-purple-600 dark:text-purple-400", background: "bg-purple-500/10" },
  { extensions: ["mp3", "wav", "ogg", "flac", "aac", "m4a", "wma", "opus", "aiff"], icon: FileAudio, label: "Audio", color: "text-amber-600 dark:text-amber-400", background: "bg-amber-500/10" },
  { extensions: ["pdf"], icon: FileText, label: "PDF document", color: "text-red-600 dark:text-red-400", background: "bg-red-500/10" },
  { extensions: ["doc", "docx", "odt", "rtf", "txt", "md", "log"], icon: FileText, label: "Text document", color: "text-blue-600 dark:text-blue-400", background: "bg-blue-500/10" },
  { extensions: ["xls", "xlsx", "csv", "ods", "tsv"], icon: FileSpreadsheet, label: "Spreadsheet", color: "text-emerald-600 dark:text-emerald-400", background: "bg-emerald-500/10" },
  { extensions: ["ppt", "pptx", "odp", "key"], icon: Presentation, label: "Presentation", color: "text-orange-600 dark:text-orange-400", background: "bg-orange-500/10" },
  { extensions: ["zip", "rar", "7z", "tar", "gz", "bz2", "xz", "tgz", "zst"], icon: FileArchive, label: "Archive", color: "text-yellow-700 dark:text-yellow-400", background: "bg-yellow-500/10" },
  { extensions: ["js", "jsx", "ts", "tsx", "py", "go", "html", "htm", "css", "scss", "json", "xml", "sql", "sh", "bash", "c", "cpp", "h", "rs", "java", "php", "rb", "vue", "svelte"], icon: FileCode, label: "Code", color: "text-indigo-600 dark:text-indigo-400", background: "bg-indigo-500/10" },
  { extensions: ["exe", "msi", "dmg", "app", "apk"], icon: FileCog, label: "Application / installer", color: "text-cyan-700 dark:text-cyan-400", background: "bg-cyan-500/10" },
  { extensions: ["env", "ini", "conf", "config", "toml", "yaml", "yml"], icon: FileSliders, label: "Configuration", color: "text-slate-600 dark:text-slate-400", background: "bg-slate-500/10" },
];

export function FileTypeIcon({ name }: { name: string }) {
  const extension = name.includes(".") ? name.split(".").pop()?.toLowerCase() || "" : "";
  const type = types.find(type => type.extensions.includes(extension));
  const Icon = type?.icon || File;
  const label = type?.label || "File";
  return <span title={extension ? `${label} (.${extension})` : label} className={`grid size-12 shrink-0 place-items-center rounded-xl ${type?.color || "text-primary"} ${type?.background || "bg-primary/10"}`}>
    <Icon className="size-7" aria-hidden="true" />
    <span className="sr-only">{label}</span>
  </span>;
}

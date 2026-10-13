"use client";
import { createPortal } from "react-dom";
import { Plus, Upload, FolderUp, FolderPlus, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
export function NewMenu({ target, disabled, uploadFiles, uploadFolder, newFolder }: { target: HTMLDivElement | null; disabled: boolean; uploadFiles: () => void; uploadFolder: () => void; newFolder: () => void }) {
 if (!target) return null;
 return createPortal(<DropdownMenu modal={false}><DropdownMenuTrigger asChild><Button disabled={disabled}><Plus />New<ChevronDown /></Button></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem onSelect={uploadFiles}><Upload />File upload</DropdownMenuItem><DropdownMenuItem onSelect={uploadFolder}><FolderUp />Folder upload</DropdownMenuItem><DropdownMenuItem onSelect={newFolder}><FolderPlus />New folder</DropdownMenuItem></DropdownMenuContent></DropdownMenu>,target);
}

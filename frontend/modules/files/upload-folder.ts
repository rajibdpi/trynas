export async function uploadFolderFiles(files: File[], destination: string, mkdir: (path: string) => Promise<void>, upload: (path: string, files: File[]) => Promise<void>) {
 const directories = new Set<string>();
 const groups = new Map<string, File[]>();
 for (const file of files) {
  const parts = file.webkitRelativePath.split("/");
  if (parts.length < 2 || parts.some(part => !part || part === "." || part === ".." || /[\\\x00]/.test(part))) throw Error("Invalid folder upload path");
  for (let i = 1; i < parts.length; i++) directories.add(parts.slice(0,i).join("/"));
  const directory = parts.slice(0,-1).join("/");
  groups.set(directory,[...(groups.get(directory) || []),file]);
 }
 const join = (path: string) => [destination,path].filter(Boolean).join("/");
 for (const directory of [...directories].sort((a,b) => a.split("/").length-b.split("/").length)) await mkdir(join(directory));
 for (const [directory, items] of groups) await upload(join(directory),items);
}

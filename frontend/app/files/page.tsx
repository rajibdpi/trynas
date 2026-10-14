import Workspace from "@/modules/workspace/workspace";

export const metadata = { title: "Files | TryNAS" };

export default function FilesPage() {
  return <Workspace initialTab="files" />;
}

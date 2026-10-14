import Workspace from "@/modules/workspace/workspace";

export const metadata = { title: "Folder permissions | TryNAS" };

export default function Page() {
  return <Workspace initialTab="permissions" />;
}

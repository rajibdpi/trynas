import Workspace from "@/modules/workspace/workspace";

export const metadata = { title: "Trash | TryNAS" };

export default function Page() {
  return <Workspace initialTab="trash" />;
}

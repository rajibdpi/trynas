import Workspace from "@/modules/workspace/workspace";

export const metadata = { title: "Dashboard | TryNAS" };

export default function Page() {
  return <Workspace initialTab="dashboard" />;
}

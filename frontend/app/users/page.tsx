import Workspace from "@/modules/workspace/workspace";

export const metadata = { title: "Users & quotas | TryNAS" };

export default function Page() {
  return <Workspace initialTab="users" />;
}

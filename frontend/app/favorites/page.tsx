import Workspace from "@/modules/workspace/workspace";

export const metadata = { title: "Favorites | TryNAS" };

export default function Page() {
  return <Workspace initialTab="favorites" />;
}

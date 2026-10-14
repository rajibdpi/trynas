import Workspace from "@/modules/workspace/workspace";

export const metadata = { title: "Settings | TryNAS" };

export default function SettingsPage() {
  return <Workspace initialTab="settings" />;
}

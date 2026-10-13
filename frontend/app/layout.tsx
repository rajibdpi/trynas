import "./globals.css";
import { ThemeProvider } from "@/modules/settings/theme-provider";
import { NotificationProvider } from "@/modules/shared/notification-provider";

export const metadata = { title: "TryNAS", description: "File Browser + NAS Dashboard" };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" translate="no">
      <head><meta name="google" content="notranslate" /></head>
      <body>
        <ThemeProvider>
          <NotificationProvider>
            {children}
          </NotificationProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}

"use client";

import Link from "next/link";
import { AppFooter } from "@/components/app-footer";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { HardDrive, RefreshCw } from "lucide-react";
import { api, ApiError } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/modules/shared/utils";
import { useNotification } from "@/modules/shared/notification-provider";
import { AppSettingsPanel, defaultSettings, type AppSettings } from "@/modules/settings/app-settings";
import { AttachedStorageBrowser } from "@/modules/storage/attached-storage-browser";
import { CommonNavbar } from "@/modules/workspace/common-navbar";
import { type TrafficSample } from "@/modules/metrics/network-traffic-graph";
import { type UsageSample, type HistorySample } from "@/modules/metrics/usage-graph";
import { type User, type Item, type Quota, type Command } from "@/modules/shared/types";
import { fmt, message, parent } from "@/modules/shared/format";
import { nav } from "@/modules/workspace/navigation";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText } from "@/modules/shared/feedback";
import { CommandDialog } from "@/modules/shared/command-dialog";
import { Overview } from "@/modules/dashboard/overview";
import { FileBrowser } from "@/modules/files/file-browser";
import { UserPanel } from "@/modules/users/user-panel";
import { PermissionPanel } from "@/modules/permissions/permission-panel";
import { SystemPanel } from "@/modules/metrics/system-panel";
import { HardwarePanel } from "@/modules/metrics/hardware-panel";
import { ActivityPanel } from "@/modules/activity/activity-panel";
import { FavoritesPanel } from "@/modules/favorites/favorites-panel";
import { SharesPanel } from "@/modules/share/shares-panel";
import { TrashPanel } from "@/modules/trash/trash-panel";
import { readFileLocation, fileLocationURL, useFileLocation, writeFileLocation } from "@/modules/files/file-location";
import { Preview } from "@/modules/files/preview";
import { LoginPanel } from "@/modules/auth/login-panel";

export default function Workspace({ initialTab = "dashboard" }: { initialTab?: string }) {
  const pathname = usePathname();
  const fileLocation = useFileLocation();
  const showNotification = useNotification();
  const notificationVersion = useRef(0);
  const [appSettings, setAppSettings] = useState<AppSettings>(defaultSettings);
  const [me, setMe] = useState<User | null>(null),
    [checking, setChecking] = useState(true),
    [username, setUsername] = useState("admin"),
    [password, setPassword] = useState(""),
    [loginError, setLoginError] = useState(""),
    [signing, setSigning] = useState(false),
    [signingOut, setSigningOut] = useState(false);
  const [tab, setTab] = useState(initialTab),
    [fileQuery, setFileQuery] = useState(""),
    [path, setPath] = useState(""),
    [pathHistory, setPathHistory] = useState<string[]>([]),
    [forwardHistory, setForwardHistory] = useState<string[]>([]),
    [mobile, setMobile] = useState(false),
    [sidebarCollapsed, setSidebarCollapsed] = useState(false),
    [dark, setDark] = useState(false),
    [command, setCommand] = useState<Command | null>(null),
    [preview, setPreview] = useState<Item | null>(null);
  useEffect(() => {
    const section = pathname.split("/").filter(Boolean)[0] || "dashboard";
    setTab(nav.some(item => item.id === section) ? section : "dashboard");
  }, [pathname]);
  const activeTab = me && me.role !== "admin" && nav.find(item => item.id === tab)?.admin ? "files" : tab;
  useEffect(() => {
    if (me && me.role !== "admin" && nav.find(item => item.id === tab)?.admin) {
      setTab("files");
      if (!(window.location.pathname === "/files" || window.location.pathname.startsWith("/files/"))) writeFileLocation(me.home || "", undefined, undefined, true);
    }
  }, [me?.role, tab]);
  const selectTab = (next: string, resetFiles = false) => {
    if (me?.role !== "admin" && nav.find(item => item.id === next)?.admin) next = "files";
    setTab(next);
    setMobile(false);
    const folder = resetFiles ? (me?.rootAccessible ? "" : (me?.home || "")) : path;
    const route = next === "files" ? fileLocationURL(folder) : `/${next}`;
    if (next === "files") {
      setPath(folder);
      setPreview(null);
      setFileQuery("");
      writeFileLocation(folder);
    }
    else if (window.location.pathname !== route) window.history.pushState(null, "", route);
  };
  useEffect(() => {
    if (!me) return;
    const controller = new AbortController();
    void api("/api/preferences", { signal: controller.signal }).then(data => setAppSettings({ ...defaultSettings, ...data.settings })).catch(() => { });
    return () => controller.abort();
  }, [me?.id]);
  useEffect(() => { document.title = appSettings.appName; }, [appSettings.appName]);
  const sys = useResource<any>(me?.role === "admin" ? "/api/system" : null, 5000),
    network = useResource<any>(me?.role === "admin" ? "/api/network" : null, 3000),
    quota = useResource<Quota>(me ? "/api/quota" : null, 5000);
  const [trafficSamples, setTrafficSamples] = useState<TrafficSample[]>([]);
  const [usageSamples, setUsageSamples] = useState<UsageSample[]>([]);
  const [storageSamples, setStorageSamples] = useState<HistorySample[]>([]);
  useEffect(() => {
    if (!me) { setStorageSamples([]); return; }
    if (!quota.data || !Number.isFinite(quota.data.used)) return;
    const at = Date.now();
    setStorageSamples(previous => [...previous.filter(sample => sample.at > at - 60000), { at, value: Math.max(0, quota.data!.used) }].slice(-60));
  }, [quota.data, me?.id]);
  useEffect(() => {
    if (!me) { setUsageSamples([]); return; }
    if (!sys.data) return;
    const cpu = sys.data.cpu;
    const memory = sys.data.memoryPercent;
    if (!Number.isFinite(cpu) || !Number.isFinite(memory)) return;
    const at = Date.now();
    setUsageSamples(previous => [...previous.filter(sample => sample.at > at - 60000), {
      at, cpu: Math.min(100, Math.max(0, cpu)), memory: Math.min(100, Math.max(0, memory)),
    }].slice(-60));
  }, [sys.data, me?.id]);
  useEffect(() => {
    if (!me) { setTrafficSamples([]); return; }
    if (!network.data?.available) return;
    const rx = network.data.rxBytesPerSecond;
    const tx = network.data.txBytesPerSecond;
    if (!Number.isFinite(rx) || !Number.isFinite(tx)) return;
    const at = Date.now();
    setTrafficSamples(previous => [...previous.filter(sample => sample.at > at - 60000), { at, rx: Math.max(0, rx), tx: Math.max(0, tx) }].slice(-60));
  }, [network.data, me?.id]);
  useEffect(() => {
    let active = true;
    void api("/api/me", { signal: AbortSignal.timeout(10000) })
      .then((u) => {
        if (active) { setMe(u); setPath(u.home || "") }
      })
      .catch(() => { })
      .finally(() => {
        if (active) setChecking(false);
      });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    const expired = () => {
      setMe(null);
      setCommand(null);
      setPreview(null);
      setLoginError("Your session expired. Please sign in again.");
    };
    window.addEventListener("trynas:unauthorized", expired);
    return () => window.removeEventListener("trynas:unauthorized", expired);
  }, []);
  useEffect(() => {
    const value = localStorage.getItem("trynas-theme");
    const enabled = value
      ? value === "dark"
      : window.matchMedia("(prefers-color-scheme: dark)").matches;
    setDark(enabled);
    document.documentElement.classList.toggle("dark", enabled);
  }, []);
  const toggleTheme = () => {
    const enabled = !dark;
    setDark(enabled);
    localStorage.setItem("trynas-theme", enabled ? "dark" : "light");
    document.documentElement.classList.toggle("dark", enabled);
  };
  const signIn = async () => {
    setSigning(true);
    try {
      const user = await api("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      setMe(user);
      setPath(user.home || "");
      setPathHistory([]); setForwardHistory([]);
      setPassword("");
      setLoginError("");
      if (window.location.pathname === "/login") {
        if (user.role === "admin") window.history.replaceState(null, "", "/dashboard");
        else writeFileLocation(user.home || "", undefined, undefined, true);
      }
    } catch (e) {
      setLoginError(
        e instanceof ApiError
          ? e.status === 401
            ? "Invalid username or password"
            : message(e)
          : "Cannot reach the server. Check that the backend is running.",
      );
    } finally {
      setSigning(false);
    }
  };
  const notify = (text: string) => { notificationVersion.current++; showNotification(text); };
  const ask = (c: Command) => setCommand({
    ...c, run: async values => {
      const before = notificationVersion.current;
      try { await c.run(values); }
      catch (error) { notify(message(error)); throw error; }
      if (notificationVersion.current === before) notify(`${c.title} completed`);
    }
  });
  useEffect(() => {
    const currentLocation = readFileLocation();
    if (!me || !currentLocation || currentLocation.storage) return;
    const nextPath = currentLocation.path ?? me.home ?? "";
    setPath(nextPath);
    setFileQuery("");
    setPreview(null);
    if (currentLocation.path === null) {
      writeFileLocation(nextPath, currentLocation.file ?? undefined, undefined, true);
      return;
    }
    if (!currentLocation.file) return;
    const controller = new AbortController();
    const file = currentLocation.file;
    void api(`/api/files?path=${encodeURIComponent(parent(file))}`, { signal: controller.signal }).then(data => {
      if (controller.signal.aborted) return;
      const item = (data.items as Item[]).find(item => item.path === file && !item.dir);
      if (!item) { showNotification("File not found or access denied."); return; }
      setPreview(item);
    }).catch(error => { if (!controller.signal.aborted) showNotification(message(error)); });
    return () => controller.abort();
  }, [me?.id, pathname, fileLocation?.path, fileLocation?.file, fileLocation?.storage]);
  const navigateFolder = (next: string) => {
    setTab("files");
    setPreview(null);
    writeFileLocation(next);
    if (next === path) return;
    setFileQuery("");
    setPathHistory(previous => [...previous, path]);
    setForwardHistory([]);
    setPath(next);
  };
  const goBack = () => {
    if (!pathHistory.length) return;
    setFileQuery("");
    setForwardHistory(previous => [...previous, path]);
    setPreview(null);
    writeFileLocation(pathHistory[pathHistory.length - 1]);
    setPath(pathHistory[pathHistory.length - 1]);
    setPathHistory(previous => previous.slice(0, -1));
  };
  const goForward = () => {
    if (!forwardHistory.length) return;
    setFileQuery("");
    setPathHistory(previous => [...previous, path]);
    setPreview(null);
    writeFileLocation(forwardHistory[forwardHistory.length - 1]);
    setPath(forwardHistory[forwardHistory.length - 1]);
    setForwardHistory(previous => previous.slice(0, -1));
  };
  const openItem = (item: Item) => {
    if (item.dir) {
      navigateFolder(item.path);
    } else {
      setTab("files");
      setPath(parent(item.path));
      setPreview(item);
      writeFileLocation(parent(item.path), item.path);
    }
  };
  const refresh = () => {
    void sys.reload();
    void quota.reload();
    void network.reload();
  };
  if (checking)
    return (
      <main className="grid min-h-screen place-items-center">
        Connecting to TryNAS…
      </main>
    );
  if (!me) return <><LoginPanel username={username} password={password} setUsername={setUsername} setPassword={setPassword} loginError={loginError} signing={signing} dark={dark} toggleTheme={toggleTheme} signIn={signIn} /><AppFooter appName={appSettings.appName} footerText={appSettings.footerText} /></>;
  let content: ReactNode = null;
  const fileProps = {
    defaultView: appSettings.fileView,
    defaultShareExpiry: appSettings.shareExpiry,
    query: fileQuery,
    setQuery: setFileQuery,
    path,
    basePath: me.rootAccessible ? "" : (me.home || ""),
    setPath: navigateFolder,
    onBack: goBack,
    canGoBack: pathHistory.length > 0,
    onForward: goForward,
    canGoForward: forwardHistory.length > 0,
    openItem,
    ask,
    notify,
    onChanged: () => void quota.reload(),
  };
  switch (activeTab) {
    case "settings":
      content = me.role === "admin" ? <AppSettingsPanel onSaved={setAppSettings} /> : <ErrorText text="Settings are available to administrators." />;
      break;
    case "dashboard":
      content = (
        <>
          <Overview sys={sys.data} quota={quota.data} network={network.data} trafficSamples={trafficSamples} networkError={network.error} usageSamples={usageSamples} systemError={sys.error} storageSamples={storageSamples} quotaError={quota.error} />
          <FileBrowser {...fileProps} />
        </>
      );
      break;
    case "files":
      content = <AttachedStorageBrowser admin={me.role === "admin"} query={fileQuery} setQuery={setFileQuery}><FileBrowser {...fileProps} /></AttachedStorageBrowser>;
      break;
    case "system":
      content = <SystemPanel sys={sys.data} network={network.data} trafficSamples={trafficSamples} networkError={network.error} />;
      break;
    case "users":
      content = (
        <UserPanel
          me={me}
          ask={ask}
          notify={notify}
          onChanged={() => {
            void quota.reload();
            void api("/api/me")
              .then((user: User) => {
                setMe(user);
                if (user.role !== "admin") setTab("dashboard");
              })
              .catch(() => { });
          }}
        />
      );
      break;
    case "permissions":
      content = <PermissionPanel ask={ask} notify={notify} />;
      break;
    case "hardware":
      content = <HardwarePanel />;
      break;
    case "activity":
      content = <ActivityPanel />;
      break;
    case "favorites":
      content = (
        <FavoritesPanel
          openPath={(p) => {
            navigateFolder(parent(p));
            selectTab("files");
          }}
          notify={notify}
        />
      );
      break;
    case "shared":
      content = <SharesPanel ask={ask} notify={notify} />;
      break;
    case "trash":
      content = (
        <TrashPanel
          ask={ask}
          notify={notify}
          onChanged={() => void quota.reload()}
        />
      );
      break;
  }
  return (
    <div className={cn("app-shell min-h-screen pt-18 md:grid", sidebarCollapsed ? "md:grid-cols-[76px_minmax(0,1fr)]" : "md:grid-cols-[260px_minmax(0,1fr)]")}>
      <CommonNavbar
        appName={appSettings.appName}
        logoUrl={appSettings.logoUrl}
        user={me}
        dark={dark}
        signingOut={signingOut}
        sidebarCollapsed={sidebarCollapsed}
        mobileOpen={mobile}
        searchQuery={fileQuery}
        onSearchQueryChange={setFileQuery}
        onSearch={() => { selectTab("files"); setMobile(false); }}
        onToggleSidebar={() => setSidebarCollapsed(value => !value)}
        onToggleTheme={toggleTheme}
        onOpenNavigation={() => setMobile(value => !value)}
        onHome={() => selectTab("dashboard")}
        onLogout={() => {
          setSigningOut(true);
          void api("/api/auth/logout", { method: "POST" })
            .then(() => {
              setMe(null);
              setPath("");
              setFileQuery("");
              setPathHistory([]);
              setForwardHistory([]);
              setTab((pathname === "/files" || pathname.startsWith("/files/")) ? "files" : "dashboard");
              setMobile(false);
              setCommand(null);
              setPreview(null);
              window.history.replaceState(null, "", "/login");
            })
            .catch((e) => notify(message(e)))
            .finally(() => setSigningOut(false));
        }}
      />
      {mobile && (
        <button
          className="fixed inset-x-0 bottom-0 top-18 z-30 bg-black/50 md:hidden"
          aria-label="Close navigation"
          onClick={() => setMobile(false)}
        />
      )}
      <aside
        id="workspace-sidebar"
        className={cn(
          "app-sidebar fixed bottom-0 top-18 left-0 z-40 flex w-65 flex-col overflow-y-auto p-5 md:sticky md:top-18 md:h-[calc(100vh-4.5rem)]",
          mobile ? "translate-x-0" : "-translate-x-full md:translate-x-0",
          sidebarCollapsed && "sidebar-collapsed md:w-19 md:px-3",
        )}
      >
        <nav
          className="flex-1 space-y-1 overflow-auto"
          aria-label="Main navigation"
        >
          {nav
            .filter((x) => !x.admin || me.role === "admin")
            .map((x) => (
              <div key={x.id}>
                {["dashboard", "system", "users"].includes(x.id) && (
                  <p className={cn("nav-group", sidebarCollapsed && "md:hidden")}>
                    {x.id === "dashboard"
                      ? "Workspace"
                      : x.id === "system"
                        ? "Monitor"
                        : "Administration"}
                  </p>
                )}
                <Link href={x.id === "files" ? fileLocationURL(me.rootAccessible ? "" : (me.home || "")) : `/${x.id}`}
                    aria-label={x.label}
                    title={sidebarCollapsed ? x.label : undefined}
                    aria-current={activeTab === x.id ? "page" : undefined}
                    className={cn(
                      "nav-link flex w-full items-center gap-3 rounded-xl px-3 py-3 text-left text-sm",
                      activeTab === x.id && "is-active",
                      sidebarCollapsed && "md:justify-center md:px-0",
                    )}
                    onClick={event => {
                      if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
                      event.preventDefault();
                      selectTab(x.id, true);
                    }}
                  >
                    <x.icon className="size-5 shrink-0" />
                    <span className={sidebarCollapsed ? "md:hidden" : undefined}>{x.label}</span>
                  </Link>

              </div>
            ))}
        </nav>
        {quota.data && (
          <div className={cn("sidebar-quota", sidebarCollapsed && "md:hidden")}>
            <div className="mb-3 flex items-center gap-2 text-xs font-medium">
              <HardDrive className="size-4" />
              Your storage
            </div>
            <Progress
              value={
                quota.data.unlimited
                  ? 0
                  : (quota.data.used / quota.data.limit) * 100
              }
            />
            <p className="mt-2 text-xs text-slate-400">
              {fmt(quota.data.used)} of{" "}
              {quota.data.unlimited ? "unlimited space" : fmt(quota.data.limit)}
            </p>
          </div>
        )}
      </aside>
      <main className="workspace flex min-h-[calc(100dvh-4.5rem)] min-w-0 flex-col p-4 md:px-9 md:py-7 xl:px-12">
        <header className="workspace-header mb-8 flex items-center gap-3">
          <div>
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-[.2em] text-muted-foreground">
              Workspace /{" "}
              {me.role === "admin" ? "Administrator" : "Personal storage"}
            </p>
            <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
              {nav.find((x) => x.id === activeTab)?.label}
            </h1>
            <p className="text-sm text-muted-foreground">
              Everything you need. In one place.
            </p>
          </div>
          <div className="server-status ml-auto hidden sm:flex">
            <span
              className={cn("status-dot", sys.error && "bg-destructive!")}
            />
            {sys.error
              ? "Server unavailable"
              : sys.data
                ? sys.data.hostname
                : "Connecting…"}
          </div>
          <Button
            variant="outline"
            size="icon"
            className="ml-auto sm:ml-2"
            aria-label="Refresh metrics"
            onClick={refresh}
          >
            <RefreshCw />
          </Button>
        </header>
        {sys.error && <ErrorText text={sys.error} />}
        <section key={activeTab}>{content}</section>
        <div className="mt-auto pt-8"><AppFooter appName={appSettings.appName} footerText={appSettings.footerText} /></div>
      </main>
      <CommandDialog command={command} close={() => setCommand(null)} />
      {preview && <Preview item={preview} close={() => { setPreview(null); writeFileLocation(path); }} />}
    </div>
  );
}

"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  Activity,
  Archive,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  Check,
  Copy,
  Download,
  Eye,
  File,
  Folder,
  Grid2X2,
  HardDrive,
  Home,
  Link2,
  List,
  MoreHorizontal,
  Moon,
  Move,
  Plus,
  RefreshCw,
  Search,
  Settings,
  Server,
  Shield,
  Star,
  Sun,
  Trash2,
  Upload,
  Users,
  X,
} from "lucide-react";
import { API, api, ApiError, uploadFiles, shareURL } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
} from "@/components/ui/context-menu";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { FolderPicker } from "@/components/folder-picker";
import { FileTypeIcon } from "@/components/file-type-icon";
import { useNotification } from "@/components/notification-provider";
import { AppSettingsPanel, defaultSettings, type AppSettings } from "@/components/app-settings";
import { CommonNavbar } from "@/components/common-navbar";
import { NetworkTrafficGraph, type TrafficSample } from "@/components/network-traffic-graph";
import { UsageGraph, HistoryGraph, type UsageSample, type HistorySample } from "@/components/usage-graph";

type User = {
 home?: string;
 rootAccessible?: boolean;
  id: number;
  username: string;
  role: string;
  quota?: number;
  used?: number;
};
type Item = {
  name: string;
  path: string;
  dir: boolean;
  size: number;
  modified: string;
  favorite?: boolean;
  writable: boolean;
};
type Quota = { used: number; limit: number; unlimited: boolean };
type Field = {
  key: string;
  label: string;
  value: string;
  type?: string;
  options?: { value: string; label: string }[];
  required?: boolean;
  disabled?: boolean;
  visibleWhen?: { key: string; value: string };
};
type Command = {
  title: string;
  description: string;
  fields: Field[];
  danger?: boolean;
  destinationFolders?: string[];
 destinationRoot?: string;
  submit: string;
  run: (values: Record<string, string>) => Promise<void>;
};
type Action = {
  label: string;
  icon: typeof Eye;
  disabled?: boolean;
  danger?: boolean;
  run: () => void;
};
const fmt = (value: number = 0) => {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  while (value >= 1024 && i < 4) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(i ? 1 : 0)} ${units[i]}`;
};
const message = (e: unknown) =>
  e instanceof Error ? e.message.trim() : "Something went wrong";
const parent = (p: string) => p.split("/").slice(0, -1).join("/");
const join = (a: string, b: string) => (a ? `${a}/${b}` : b);
const nav = [
  { id: "dashboard", label: "Dashboard", icon: Home },
  { id: "files", label: "Files", icon: Folder },
  { id: "favorites", label: "Favorites", icon: Star },
  { id: "shared", label: "Shared links", icon: Link2 },
  { id: "trash", label: "Trash", icon: Trash2 },
  { id: "system", label: "System & network", icon: Server },
  { id: "activity", label: "Activity", icon: Activity },
  { id: "users", label: "Users & quotas", icon: Users, admin: true },
  { id: "permissions", label: "Folder permissions", icon: Shield, admin: true },
  { id: "settings", label: "Settings", icon: Settings, admin: true },
  { id: "hardware", label: "Disk health", icon: HardDrive, admin: true },
];

function useResource<T>(url: string | null, interval = 0) {
  const [data, setData] = useState<T | null>(null),
    [error, setError] = useState("");
  const reload = useCallback(
    async (overrideUrl?: string) => {
      if (!url) return;
      try {
        setData(await api(overrideUrl || url));
        setError("");
      } catch (e) {
        setError(message(e));
      }
    },
    [url],
  );
  useEffect(() => {
    if (!url) {
      setData(null);
      setError("");
      return;
    }
    let active = true;
    const controller = new AbortController();
    const fetchData = async () => {
      try {
        const result = await api(url, { signal: controller.signal });
        if (active) {
          setData(result);
          setError("");
        }
      } catch (e) {
        if (active) setError(message(e));
      }
    };
    void fetchData();
    const timer = interval
      ? setInterval(() => void fetchData(), interval)
      : null;
    return () => {
      active = false;
      controller.abort();
      if (timer) clearInterval(timer);
    };
  }, [url, interval]);
  return { data, error, reload };
}
export default function Workspace({ initialTab = "dashboard" }: { initialTab?: string }) {
  const pathname = usePathname();
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
    if (pathname === "/files") setTab("files");
    else if (pathname === "/settings") setTab("settings");
    else setTab(previous => ["files", "settings"].includes(previous) ? "dashboard" : previous);
  }, [pathname]);
  const selectTab = (next: string) => {
    setTab(next);
    setMobile(false);
    const route = ["files", "settings"].includes(next) ? `/${next}` : "/";
    if (window.location.pathname !== route) window.history.pushState(null, "", route);
  };
  useEffect(() => {
    if (!me) return;
    const controller = new AbortController();
    void api("/api/settings", { signal: controller.signal }).then(data => setAppSettings(data.settings)).catch(() => {});
    return () => controller.abort();
  }, [me?.id]);
  useEffect(() => { document.title = appSettings.appName; }, [appSettings.appName]);
  const sys = useResource<any>(me ? "/api/system" : null, 5000),
    network = useResource<any>(me ? "/api/network" : null, 3000),
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
    void api("/api/me")
      .then((u) => {
        if (active) {setMe(u);setPath(u.home||"")}
      })
      .catch(() => {})
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
      setPath(user.home||"");
      setPathHistory([]);setForwardHistory([]);
      setPassword("");
      setLoginError("");
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
  const ask = (c: Command) => setCommand({ ...c, run: async values => {
    const before = notificationVersion.current;
    try { await c.run(values); }
    catch (error) { notify(message(error)); throw error; }
    if (notificationVersion.current === before) notify(`${c.title} completed`);
  } });
  const navigateFolder = (next: string) => {
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
    setPath(pathHistory[pathHistory.length - 1]);
    setPathHistory(previous => previous.slice(0, -1));
  };
  const goForward = () => {
    if (!forwardHistory.length) return;
    setFileQuery("");
    setPathHistory(previous => [...previous, path]);
    setPath(forwardHistory[forwardHistory.length - 1]);
    setForwardHistory(previous => previous.slice(0, -1));
  };
  const openItem = (item: Item) => {
    if (item.dir) {
      navigateFolder(item.path);
      selectTab("files");
    } else setPreview(item);
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
  if (!me)
    return (
      <main className="login-screen grid min-h-screen place-items-center p-4">
        <Card className="w-full max-w-md">
          <CardHeader>
            <div className="flex items-center gap-2 text-xl font-bold text-primary">
              <HardDrive />
              TryNAS <span className="text-xs">V5</span>
            </div>
            <CardTitle className="mt-4 text-2xl">Welcome back</CardTitle>
            <p className="text-sm text-muted-foreground">
              Sign in to your private NAS.
            </p>
          </CardHeader>
          <CardContent>
            <form
              className="space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                void signIn();
              }}
            >
              <label className="block text-sm">
                Username
                <input
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                />
              </label>
              <label className="block text-sm">
                Password
                <input
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </label>
              {loginError && (
                <p role="alert" className="text-sm text-destructive">
                  {loginError}
                </p>
              )}
              <Button className="w-full" disabled={signing}>
                {signing ? "Signing in…" : "Sign in"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="w-full"
                onClick={toggleTheme}
              >
                {dark ? <Sun /> : <Moon />}Change appearance
              </Button>
            </form>
          </CardContent>
        </Card>
      </main>
    );
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
  switch (tab) {
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
      content = <FileBrowser {...fileProps} />;
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
              .catch(() => {});
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
              setTab(pathname === "/files" ? "files" : "dashboard");
              setMobile(false);
              setCommand(null);
              setPreview(null);
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
          "app-sidebar fixed bottom-0 top-18 left-0 z-40 flex w-[260px] flex-col overflow-y-auto p-5 md:sticky md:top-18 md:h-[calc(100vh-4.5rem)]",
          mobile ? "translate-x-0" : "-translate-x-full md:translate-x-0",
          sidebarCollapsed && "sidebar-collapsed md:w-[76px] md:px-3",
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
                {["files", "settings"].includes(x.id) ? (
                <Link href={`/${x.id}`}
                  aria-label={x.label}
                  title={sidebarCollapsed ? x.label : undefined}
                  aria-current={tab === x.id ? "page" : undefined}
                  className={cn(
                    "nav-link flex w-full items-center gap-3 rounded-xl px-3 py-3 text-left text-sm",
                    tab === x.id && "is-active",
                    sidebarCollapsed && "md:justify-center md:px-0",
                  )}
                  onClick={event => {
                    if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
                    event.preventDefault();
                    selectTab(x.id);
                  }}
                >
                  <x.icon className="size-5 shrink-0" />
                  <span className={sidebarCollapsed ? "md:hidden" : undefined}>{x.label}</span>
                </Link>
                ) : (
                <button
                  aria-label={x.label}
                  title={sidebarCollapsed ? x.label : undefined}
                  aria-current={tab === x.id ? "page" : undefined}
                  className={cn(
                    "nav-link flex w-full items-center gap-3 rounded-xl px-3 py-3 text-left text-sm",
                    tab === x.id && "is-active",
                    sidebarCollapsed && "md:justify-center md:px-0",
                  )}
                  onClick={() => {
                    selectTab(x.id);
                  }}
                >
                  <x.icon className="size-5 shrink-0" />
                  <span className={sidebarCollapsed ? "md:hidden" : undefined}>{x.label}</span>
                </button>
                )}
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
      <main className="workspace min-w-0 p-4 md:px-9 md:py-7 xl:px-12">
        <header className="workspace-header mb-8 flex items-center gap-3">
          <div>
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-[.2em] text-muted-foreground">
              Workspace /{" "}
              {me.role === "admin" ? "Administrator" : "Personal storage"}
            </p>
            <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
              {nav.find((x) => x.id === tab)?.label}
            </h1>
            <p className="text-sm text-muted-foreground">
              Everything you need. In one place.
            </p>
          </div>
          <div className="server-status ml-auto hidden sm:flex">
            <span
              className={cn("status-dot", sys.error && "!bg-destructive")}
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
        <section key={tab}>{content}</section>
      </main>
      <CommandDialog command={command} close={() => setCommand(null)} />
      {preview && <Preview item={preview} close={() => setPreview(null)} />}
    </div>
  );
}
function ErrorText({ text }: { text: string }) {
  return text ? (
    <p
      role="alert"
      className="my-3 rounded border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
    >
      {text}
    </p>
  ) : null;
}
function Empty({ text }: { text: string }) {
  return (
    <div className="empty-state">
      <span className="empty-icon">
        <Folder className="size-7" />
      </span>
      <p className="mt-4 text-sm font-medium">{text}</p>
      <p className="mt-1 text-xs text-muted-foreground">
        Your workspace is ready when you are.
      </p>
    </div>
  );
}
function CommandDialog({
  command,
  close,
}: {
  command: Command | null;
  close: () => void;
}) {
  const [values, setValues] = useState<Record<string, string>>({}),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [destinationValid, setDestinationValid] = useState(false);
  useEffect(() => {
    setValues(
      Object.fromEntries(command?.fields.map((f) => [f.key, f.value]) || []),
    );
    setError("");
    setDestinationValid(false);
  }, [command]);
  return (
    <Dialog
      open={!!command}
      onOpenChange={(open) => {
        if (!open && !busy) close();
      }}
    >
      <DialogContent
        onEscapeKeyDown={(e) => {
          if (busy) e.preventDefault();
        }}
        onPointerDownOutside={(e) => {
          if (busy) e.preventDefault();
        }}
      >
        <DialogTitle className="pr-6 text-xl font-semibold">
          {command?.title}
        </DialogTitle>
        <DialogDescription className="mt-2 text-sm text-muted-foreground">
          {command?.description}
        </DialogDescription>
        <form
          className="mt-5 space-y-4"
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              !command ||
              busy ||
              (command.destinationFolders && !destinationValid)
            )
              return;
            setBusy(true);
            setError("");
            try {
              await command.run(values);
              close();
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          {command?.destinationFolders && (
            <FolderPicker
              key={command.title}
              value={values.destination ?? command.fields[0].value}
              excluded={command.destinationFolders}
              basePath={command.destinationRoot || ""}
              disabled={busy}
              onChange={(destination) => {
                setDestinationValid(false);
                setValues((old) => ({ ...old, destination }));
              }}
              onValidity={setDestinationValid}
            />
          )}
          {command?.fields
            .filter(
              (f) => !(command.destinationFolders && f.key === "destination") &&
                (!f.visibleWhen || (values[f.visibleWhen.key] ?? command.fields.find(field => field.key === f.visibleWhen!.key)?.value) === f.visibleWhen.value),
            )
            .map((f) => (
              <label key={f.key} className="block text-sm font-medium">
                <span className="mb-1 block">{f.label}</span>
                {f.options ? (
                  <select
                    disabled={f.disabled}
                    value={values[f.key] ?? f.value}
                    onChange={(e) =>
                      setValues({ ...values, [f.key]: e.target.value })
                    }
                  >
                    {f.options.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    disabled={f.disabled}
                    type={f.type || "text"}
                    value={values[f.key] ?? f.value}
                    onChange={(e) =>
                      setValues({ ...values, [f.key]: e.target.value })
                    }
                    required={f.required}
                    min={f.type === "number" ? 0 : undefined}
                    step={f.type === "number" ? "any" : undefined}
                    autoComplete={
                      f.type === "password" ? "new-password" : "off"
                    }
                  />
                )}
              </label>
            ))}
          <ErrorText text={error} />
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={close}
            >
              Cancel
            </Button>
            <Button
              variant={command?.danger ? "destructive" : "default"}
              disabled={
                busy || (!!command?.destinationFolders && !destinationValid)
              }
            >
              {busy ? "Working…" : command?.submit}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
function Metric({
  title,
  value,
  percent,
  children,
}: {
  title: string;
  value: string;
  percent?: number;
  children?: ReactNode;
}) {
  return (
    <Card className="metric-card">
      <CardHeader>
        <div className="mb-2 flex items-center gap-3">
          <span className="metric-icon">
            {title === "CPU" ? (
              <Activity />
            ) : title === "Memory" ? (
              <Server />
            ) : title === "Your storage" ? (
              <HardDrive />
            ) : (
              <Activity />
            )}
          </span>
          <p className="text-xs font-medium text-muted-foreground">{title}</p>
        </div>
        <CardTitle className="mt-1 text-xl font-semibold tracking-tight xl:text-2xl">
          {value}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {percent !== undefined && <Progress value={percent} />}
        {children}
      </CardContent>
    </Card>
  );
}
function Overview({
  sys,
  quota,
  network,
  trafficSamples,
  networkError,
  usageSamples,
  systemError,
  storageSamples,
  quotaError,
}: {
  sys: any;
  quota: Quota | null;
  network: any;
  trafficSamples: TrafficSample[];
  networkError: string;
  usageSamples: UsageSample[];
  systemError: string;
  storageSamples: HistorySample[];
  quotaError: string;
}) {
  return (
    <div className="mb-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Metric
        title="CPU"
        value={systemError ? "Unavailable" : sys ? `${sys.cpu.toFixed(1)}%` : "…"}
        percent={systemError ? undefined : sys?.cpu}
      >
        <UsageGraph samples={usageSamples} metric="cpu" unavailable={!!systemError} />
      </Metric>
      <Metric
        title="Memory"
        value={systemError ? "Unavailable" : sys ? `${sys.memoryPercent.toFixed(1)}%` : "…"}
        percent={systemError ? undefined : sys?.memoryPercent}
      >
        <UsageGraph samples={usageSamples} metric="memory" unavailable={!!systemError} />
      </Metric>
      <Metric
        title="Your storage"
        value={
          quotaError ? "Unavailable" : quota
            ? `${fmt(quota.used)} / ${quota.unlimited ? "Unlimited" : fmt(quota.limit)}`
            : "…"
        }
        percent={
          !quotaError && quota && !quota.unlimited && quota.limit > 0
            ? (quota.used / quota.limit) * 100
            : undefined
        }
      >
        <HistoryGraph samples={storageSamples} label="Your storage" unavailable={!!quotaError} formatValue={fmt} maxValue={quota && !quota.unlimited ? quota.limit : Math.max(1, ...storageSamples.map(sample => sample.value)) * 1.15} color="#f59e0b" />
      </Metric>
      <Metric
        title="Network traffic"
        value={networkError || network?.available === false ? "Unavailable" : network ? `↓ ${fmt(network.rxBytesPerSecond)}/s` : "…"}
      >
        <NetworkTrafficGraph samples={trafficSamples} unavailable={!!networkError || network?.available === false} />
      </Metric>
    </div>
  );
}

function FileBrowser({
  defaultView,
  defaultShareExpiry,
  query,
  setQuery,
  path,
  basePath,
  setPath,
  onBack,
  canGoBack,
  onForward,
  canGoForward,
  openItem,
  ask,
  notify,
  onChanged,
}: {
  defaultView: "list" | "grid";
  defaultShareExpiry: string;
  query: string;
  setQuery: (value: string) => void;
  path: string;
  basePath: string;
  setPath: (p: string) => void;
  onBack: () => void;
  canGoBack: boolean;
  onForward: () => void;
  canGoForward: boolean;
  openItem: (item: Item) => void;
  ask: (c: Command) => void;
  notify: (s: string) => void;
  onChanged: () => void;
}) {
  const listing = useResource<{ items: Item[]; writable: boolean }>(
    "/api/files?path=" + encodeURIComponent(path),
  );
  const [debounced, setDebounced] = useState(query.trim()),
    [view, setView] = useState(defaultView),
    [selected, setSelected] = useState<string[]>([]),
    [upload, setUpload] = useState<{
      loaded: number;
      total: number;
      stage: string;
    } | null>(null),
    [uploadError, setUploadError] = useState("");
  useEffect(() => setView(defaultView), [defaultView]);
  const search = useResource<Item[]>(
    debounced.length >= 2
      ? `/api/files/search?q=${encodeURIComponent(debounced)}&path=${encodeURIComponent(path)}`
      : null,
  );
  const input = useRef<HTMLInputElement>(null),
    selectionAnchor = useRef<string | null>(null),
    controller = useRef<AbortController | null>(null),
    dragDepth = useRef(0);
  const [draggingFiles, setDraggingFiles] = useState(false);
  const resetDrag = () => { dragDepth.current = 0; setDraggingFiles(false); };
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(query.trim()), 250);
    return () => clearTimeout(timer);
  }, [query]);
  useEffect(() => {
    setSelected([]);
    selectionAnchor.current = null;
  }, [path]);
  useEffect(() => {
    setSelected([]);
    selectionAnchor.current = null;
  }, [debounced]);
  useEffect(() => () => controller.current?.abort(), []);
  const items =
    debounced.length >= 2 ? search.data || [] : listing.data?.items || [];
  useEffect(() => {
    const visible = new Set(
      (debounced.length >= 2 ? search.data : listing.data?.items)?.map(
        (item) => item.path,
      ) || [],
    );
    setSelected((old) => old.filter((p) => visible.has(p)));
    if (selectionAnchor.current && !visible.has(selectionAnchor.current)) selectionAnchor.current = null;
  }, [listing.data, search.data, debounced]);
  const writable = listing.data?.writable || false;
  const reload = async () => {
    await listing.reload();
    await search.reload();
    onChanged();
  };
  const perform = async (action: string, paths: string[], destination = "") => {
    const result = await api("/api/files/bulk", {
      method: "POST",
      body: JSON.stringify({ action, paths, destination }),
    });
    const failed = result.results.filter((x: any) => !x.ok);
    notify(
      failed.length
        ? failed.map((x: any) => `${x.path}: ${x.error}`).join("\n")
        : `${result.results.length} item(s) ${ ({ copy: "copied", move: "moved", trash: "moved to trash", zip: "archived", favorite: "added to favorites" } as Record<string, string>)[action] || "updated" }`,
    );
    setSelected(failed.map((x: any) => x.path));
    await reload();
  };
  const bulk = (action: string, paths = selected) => {
    if (!paths.length) return;
    if (action === "copy" || action === "move")
      ask({
        title: `${action === "copy" ? "Copy" : "Move"} ${paths.length} item(s)`,
        description:
          "Expand the folder tree and select where to place the items. Read-only and source folders cannot be selected. Existing files will not be overwritten.",
        fields: [
          { key: "destination", label: "Destination folder", value: path },
        ],
        destinationRoot: basePath,
        destinationFolders: paths.filter((p) =>
          items.some((item) => item.path === p && item.dir),
        ),
        submit: action === "copy" ? "Copy here" : "Move here",
        run: (v) => perform(action, paths, v.destination),
      });
    else if (action === "trash")
      ask({
        title: `Move ${paths.length} item(s) to trash?`,
        description:
          "Files still count toward their owner's quota until permanently deleted.",
        fields: [],
        danger: true,
        submit: "Move to trash",
        run: () => perform(action, paths),
      });
    else void perform(action, paths).catch((e) => notify(message(e)));
  };
  const shareSelection = (paths: string[]) =>
        ask({
          title: "Create share link",
          description:
            "Create one link for all selected files and folders (ZIP). Add an optional password and choose an expiry in your local time. Access follows your folder permissions.",
          fields: [
            {
              key: "expiry",
              label: "Expires at",
              value: defaultShareExpiry,
              options: [
                { value: "never", label: "Never" },
                { value: "1", label: "1 day" },
                { value: "7", label: "7 days" },
                { value: "15", label: "15 days" },
                { value: "30", label: "30 days" },
                { value: "custom", label: "Custom" },
              ],
            },
            {
              key: "expiresAt",
              label: "Expires at (local date and time)",
              type: "datetime-local",
              value: (() => { const expiry = new Date(Date.now() + 7 * 86400000); return new Date(expiry.getTime() - expiry.getTimezoneOffset() * 60000).toISOString().slice(0, 16); })(),
              required: true,
              visibleWhen: { key: "expiry", value: "custom" },
            },
            { key: "password", label: "Password (optional, 8–72 bytes)", type: "password", value: "" },
          ],
          submit: "Create link",
          run: async (v) => {
            let settings: { never?: boolean; days?: number; expiresAt?: string };
            if (v.expiry === "never") settings = { never: true };
            else if (v.expiry === "custom") {
              const expiry = new Date(v.expiresAt);
              if (!Number.isFinite(expiry.getTime()) || expiry.getTime() <= Date.now())
                throw new Error("Choose a future expiry date and time");
              settings = { expiresAt: expiry.toISOString() };
            } else settings = { days: Number(v.expiry) };
            const result = await api("/api/shares", {
              method: "POST",
              body: JSON.stringify({ paths, ...settings, password: v.password }),
            });
            notify(`Share link: ${shareURL(result.token)}`);
          },
        });
  const fileActions = (item: Item): Action[] => [
    {
      label: item.dir ? "Open folder" : "Preview",
      icon: Eye,
      run: () => openItem(item),
    },
    {
      label: "Download",
      icon: Download,
      run: () => {
        window.location.href =
          API + "/api/files/download?path=" + encodeURIComponent(item.path);
      },
    },
    { label: "Favorite", icon: Star, run: () => bulk("favorite", [item.path]) },
    {
      label: "Rename",
      icon: File,
      disabled: !item.writable,
      run: () =>
        ask({
          title: "Rename item",
          description: item.path,
          fields: [
            {
              key: "name",
              label: "New name",
              value: item.name,
              required: true,
            },
          ],
          submit: "Rename",
          run: async (v) => {
            if (
              v.name.includes("/") ||
              v.name.includes("\\") ||
              v.name === "." ||
              v.name === ".."
            )
              throw new Error("Enter a single file or folder name");
            await api("/api/files/rename", {
              method: "POST",
              body: JSON.stringify({
                from: item.path,
                to: join(parent(item.path), v.name),
              }),
            });
            await reload();
          },
        }),
    },
    { label: "Copy", icon: Copy, run: () => bulk("copy", [item.path]) },
    {
      label: "Move",
      icon: Move,
      disabled: !item.writable,
      run: () => bulk("move", [item.path]),
    },
    { label: "Create ZIP", icon: Archive, run: () => bulk("zip", [item.path]) },
    {
      label: "Share link",
      icon: Link2,
      run: () => shareSelection([item.path]),
    },
    {
      label: "Move to trash",
      icon: Trash2,
      disabled: !item.writable,
      danger: true,
      run: () => bulk("trash", [item.path]),
    },
  ];
  const send = async (files: File[]) => {
    if (!files.length || upload) return;
    if (!writable) {
      notify("This folder is read-only");
      return;
    }
    const data = new FormData();
    data.append("path", path);
    files.forEach((f) => data.append("files", f));
    controller.current = new AbortController();
    setUploadError("");
    setUpload({
      loaded: 0,
      total: files.reduce((n, f) => n + f.size, 0),
      stage: "Uploading",
    });
    try {
      const result = await uploadFiles(
        data,
        (loaded, total) =>
          setUpload({
            loaded,
            total,
            stage: loaded === total ? "Processing" : "Uploading",
          }),
        controller.current.signal,
      );
      notify(`Uploaded ${result.files} file(s), ${fmt(result.bytes)}`);
      await reload();
    } catch (e) {
      setUploadError(message(e));
    } finally {
      setUpload(null);
      controller.current = null;
      if (input.current) input.current.value = "";
    }
  };
  const selectItem = (p: string, event: { ctrlKey: boolean; shiftKey: boolean }) => {
    const start = items.findIndex(item => item.path === selectionAnchor.current);
    const end = items.findIndex(item => item.path === p);
    if (event.shiftKey && start >= 0 && end >= 0) {
      const range = items.slice(Math.min(start, end), Math.max(start, end) + 1).map(item => item.path);
      setSelected(old => event.ctrlKey ? [...new Set([...old, ...range])] : range);
      return;
    }
    selectionAnchor.current = p;
    if (event.shiftKey) { setSelected([p]); return; }
    if (!event.ctrlKey) return;
    setSelected(old => old.includes(p) ? old.filter(path => path !== p) : [...old, p]);
  };
  return (
    <Card
      className={cn("file-browser relative", draggingFiles && "ring-2 ring-primary ring-offset-2 ring-offset-background")}
      onDragEnter={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        e.preventDefault();
        dragDepth.current += 1;
        setDraggingFiles(true);
      }}
      onDragOver={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = writable && !upload ? "copy" : "none";
      }}
      onDragLeave={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        dragDepth.current = Math.max(0, dragDepth.current - 1);
        if (!dragDepth.current) setDraggingFiles(false);
      }}
      onDragEnd={resetDrag}
      onDrop={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        e.preventDefault();
        resetDrag();
        void send(Array.from(e.dataTransfer.files));
      }}
    >
      {draggingFiles && <div role="status" className="pointer-events-none absolute inset-0 z-20 grid place-items-center rounded-2xl bg-card/95 p-6 backdrop-blur-sm">
        <div className="flex flex-col items-center text-center">
          <span className="mb-4 grid size-16 place-items-center rounded-2xl bg-primary/10 text-primary"><Upload className="size-8"/></span>
          <p className="text-xl font-semibold">{!writable ? "This folder is read-only" : upload ? "An upload is already in progress" : "Drop files to upload"}</p>
          <p className="mt-2 break-all text-sm text-muted-foreground">{!writable ? "Choose a folder with write access." : upload ? "Wait for the current upload to finish." : `Upload to /${path || " (storage root)"}`}</p>
        </div>
      </div>}
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <CardTitle>
                {path ? path.split("/").pop() : "All files"}
              </CardTitle>
              <span className="file-count">{items.length} items</span>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
              <Button type="button" variant="ghost" size="icon" aria-label="Back to previous folder" title="Back" disabled={!canGoBack} onClick={onBack}><ArrowLeft/></Button>
              <Button type="button" variant="ghost" size="icon" aria-label="Forward to next folder" title="Forward" disabled={!canGoForward} onClick={onForward}><ArrowRight/></Button>
              <Button type="button" variant="ghost" size="icon" aria-label="Up to parent folder" title="Up one folder" disabled={!path || path===basePath} onClick={()=>setPath(parent(path))}><ArrowUp/></Button>
              <button onClick={() => setPath(basePath)}>{basePath?"Home":"Storage"}</button>
              {path
                .split("/")
                .filter(Boolean)
                .map((part, i) => (
                  path.split("/").slice(0,i+1).join("/").length <= basePath.length ? null : <span key={i}>
                    {" "}
                    /{" "}
                    <button
                      onClick={() =>
                        setPath(
                          path
                            .split("/")
                            .slice(0, i + 1)
                            .join("/"),
                        )
                      }
                    >
                      {part}
                    </button>
                  </span>
                ))}
            </div>
          </div>
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
          <div className="relative min-w-0 flex-1 sm:w-64 sm:flex-none">
            <Search className="absolute left-3 top-3 size-4 text-muted-foreground" />
            <input
              className="!pl-9"
              placeholder="Search files and folders…"
              aria-label="Search files"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
            <Button
              variant="outline"
              disabled={!writable || !!upload}
              onClick={() => input.current?.click()}
            >
              <Upload />
              <span>Upload</span>
            </Button>
            <input
              ref={input}
              type="file"
              className="hidden"
              multiple
              onChange={(e) => void send(Array.from(e.target.files || []))}
            />
            <Button
              disabled={!writable}
              onClick={() =>
                ask({
                  title: "New folder",
                  description: "Create a folder in the current location.",
                  fields: [
                    {
                      key: "name",
                      label: "Folder name",
                      value: "",
                      required: true,
                    },
                  ],
                  submit: "Create",
                  run: async (v) => {
                    if (
                      /[\\/]/.test(v.name) ||
                      v.name === "." ||
                      v.name === ".."
                    )
                      throw new Error("Enter a single folder name");
                    await api("/api/files/folder", {
                      method: "POST",
                      body: JSON.stringify({ path: join(path, v.name) }),
                    });
                    await reload();
                  },
                })
              }
            >
              <Plus />
              <span>Folder</span>
            </Button>
            <Button
              variant="outline"
              size="icon"
              aria-label="Refresh files"
              onClick={() => void reload()}
            >
              <RefreshCw />
            </Button>
            <div className="flex gap-2" role="group" aria-label="File view">
          <Button
            variant={view === "list" ? "secondary" : "outline"}
            size="icon"
            aria-label="List view"
            aria-pressed={view === "list"}
            onClick={() => setView("list")}
          >
            <List />
          </Button>
          <Button
            variant={view === "grid" ? "secondary" : "outline"}
            size="icon"
            aria-label="Grid view"
            aria-pressed={view === "grid"}
            onClick={() => setView("grid")}
          >
            <Grid2X2 />
          </Button>
        </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <ErrorText text={listing.error || search.error || uploadError} />
        {!writable && (
          <p className="mb-3 text-xs text-muted-foreground">
            Read-only folder. An administrator can grant write access.
          </p>
        )}
        {upload && (
          <div className="mb-4 rounded-lg border p-3">
            <div className="mb-2 flex items-center justify-between text-sm">
              <span>
                {upload.stage} · {fmt(upload.loaded)} / {fmt(upload.total)}
              </span>
              <Button
                variant="ghost"
                disabled={upload.stage === "Processing"}
                onClick={() => controller.current?.abort()}
              >
                Cancel
              </Button>
            </div>
            <Progress
              value={upload.total ? (upload.loaded / upload.total) * 100 : 0}
            />
            <p className="mt-2 text-xs text-muted-foreground">
              {upload.stage === "Processing"
                ? "Transfer complete. The server is checking permissions, quota, and saving files."
                : "Progress measures bytes transferred to the server."}
            </p>
          </div>
        )}
        <div
          className={cn(
            "selection-toolbar mb-3 flex flex-wrap items-center gap-2",
            selected.length > 0 && "has-selection",
          )}
        >
          <span className="mr-2 text-sm text-muted-foreground">
            {selected.length ? `${selected.length} selected` : "Ctrl-click to select files and folders"}
          </span>
          {selected.length > 0 && (
            <>
              <Button
                variant="outline"
                size="sm"
                disabled={items.every(item => selected.includes(item.path))}
                onClick={() => setSelected(items.map(item => item.path))}
              >
                Select all
              </Button>
              <Button variant="outline" size="sm" onClick={() => shareSelection([...selected])}>
                <Link2 />
                Share
              </Button>
              <Button variant="outline" size="sm" onClick={() => bulk("copy")}>
                <Copy />
                Copy
              </Button>
              <Button variant="outline" size="sm" onClick={() => bulk("move")}>
                <Move />
                Move
              </Button>
              <Button variant="outline" size="sm" onClick={() => bulk("zip")}>
                <Archive />
                ZIP
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => bulk("favorite")}
              >
                <Star />
                Favorite
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => bulk("trash")}
              >
                <Trash2 />
                Trash
              </Button>
              <Button variant="ghost" size="sm" onClick={() => { setSelected([]); selectionAnchor.current = null; }}>
                Clear
              </Button>
            </>
          )}
        </div>
        {view === "list" && items.length > 0 && (
          <div className="file-column-head">
            <span>Name</span>
            <span>Modified</span>
            <span />
          </div>
        )}
        <div
          className={
            view === "grid"
              ? "grid grid-cols-2 gap-3 lg:grid-cols-4"
              : "space-y-1"
          }
        >
          {items.map((item) => (
            <ContextMenu key={item.path}>
              <ContextMenuTrigger asChild>
                <div
                  data-selected={selected.includes(item.path)}
                  onClick={(event) => {
                    const target = event.target as HTMLElement;
                    if (
                      event.detail > 1 ||
                      !event.currentTarget.contains(target) ||
                      target.closest("button, input, label, a, [role='menuitem']")
                    ) return;
                    selectItem(item.path, event);
                  }}
                  onDoubleClick={(event) => {
                    const target = event.target as HTMLElement;
                    if (
                      !event.currentTarget.contains(target) ||
                      target.closest("button, input, label, a, [role='menuitem']")
                    ) return;
                    openItem(item);
                  }}
                  className={cn(
                    "file-item cursor-pointer select-none rounded-xl p-3",
                    view === "grid" ? "file-tile border" : "file-row",
                    selected.includes(item.path) &&
                      "border-primary bg-primary/5",
                    view === "list" && "flex items-center gap-3",
                  )}
                >
                  <div
                    className={cn(
                      "flex items-center gap-3",
                      view === "list" ? "min-w-0 flex-1" : "mb-3",
                    )}
                  >
                    <span className="relative shrink-0">
                    {item.dir ? (
                      <span className="folder-icon">
                        <Folder className="size-7" />
                      </span>
                    ) : (
                      <FileTypeIcon name={item.name} />
                    )}
                    {selected.includes(item.path) && (
                      <span className="pointer-events-none absolute -right-1 -top-1 grid size-5 place-items-center rounded-full bg-primary text-primary-foreground ring-2 ring-card">
                        <Check className="size-3" strokeWidth={3} aria-hidden="true" />
                        <span className="sr-only">Selected</span>
                      </span>
                    )}
                    </span>
                    <button
                      className="min-w-0 text-left"
                      aria-pressed={selected.includes(item.path)}
                      title={`Double-click to open ${item.name}`}
                      onDoubleClick={() => openItem(item)}
                      onClick={(event) => {
                        if (event.detail === 0) openItem(item);
                        else if (event.detail === 1) selectItem(item.path, event);
                      }}
                    >
                      <span className="block truncate text-sm font-medium">
                        {item.name}
                      </span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {debounced.length >= 2
                          ? item.path
                          : item.dir
                            ? "Folder"
                            : fmt(item.size)}
                      </span>
                    </button>
                    {item.favorite && (
                      <Star className="size-3 text-amber-500" />
                    )}
                  </div>
                  <div className="file-meta flex items-center justify-between gap-2">
                    <span className="text-xs text-muted-foreground">
                      {new Date(item.modified).toLocaleDateString()}
                    </span>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={`Actions for ${item.name}`}
                        >
                          <MoreHorizontal />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        {fileActions(item).map((action) => (
                          <DropdownMenuItem
                            key={action.label}
                            disabled={action.disabled}
                            className={action.danger ? "text-destructive" : ""}
                            onSelect={action.run}
                          >
                            <action.icon className="size-4" />
                            <span>{action.label}</span>
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </div>
              </ContextMenuTrigger>
              <ContextMenuContent>
                {fileActions(item).map((action) => (
                  <ContextMenuItem
                    key={action.label}
                    disabled={action.disabled}
                    className={action.danger ? "text-destructive" : ""}
                    onSelect={action.run}
                  >
                    <action.icon className="size-4" />
                    <span>{action.label}</span>
                  </ContextMenuItem>
                ))}
              </ContextMenuContent>
            </ContextMenu>
          ))}
        </div>
        {!items.length && !listing.error && (
          <Empty
            text={
              debounced.length >= 2
                ? "No matching files."
                : !listing.data
                  ? "Loading your files…"
                  : "No accessible files in this folder."
            }
          />
        )}
        <div className="upload-hint mt-5 flex flex-wrap items-center justify-center gap-2 rounded-xl border border-dashed p-4 text-center text-xs text-muted-foreground">
          <Upload className="size-4" />
          <span>Drag and drop files to upload</span>
          <span className="hidden sm:inline">
            · Right-click for more actions
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
function UserPanel({
  me,
  ask,
  notify,
  onChanged,
}: {
  me: User;
  ask: (c: Command) => void;
  notify: (s: string) => void;
  onChanged: () => void;
}) {
  const users = useResource<User[]>("/api/users");
  const edit = (user?: User) =>
    ask({
      title: user ? "Edit user" : "Create user",
      description:
        "Quota 0 means unlimited. New users get a home folder named after their username with write access. Password changes revoke their sessions.",
      fields: [
        {
          key: "username",
          label: "Username",
          value: user?.username || "",
          required: true,
        },
        {
          key: "password",
          label: user
            ? "New password (blank to keep)"
            : "Password (8–72 bytes)",
          type: "password",
          value: "",
          required: !user,
        },
        {
          key: "role",
          label: "Role",
          value: user?.role || "user",
          options: [
            { value: "user", label: "User" },
            { value: "admin", label: "Administrator" },
          ],
        },
        {
          key: "quota",
          label: "Quota in GiB (0 = unlimited)",
          type: "number",
          value: String((user?.quota || 0) / 1073741824),
          required: true,
        },
      ],
      submit: "Save user",
      run: async (v) => {
        const quota = Math.round(Number(v.quota) * 1073741824);
        if (!Number.isSafeInteger(quota) || quota < 0)
          throw new Error("Enter a valid quota");
        await api("/api/users", {
          method: "POST",
          body: JSON.stringify({
            id: user?.id || 0,
            username: v.username,
            password: v.password,
            role: v.role,
            quota,
          }),
        });
        await users.reload();
        notify("User saved");
        onChanged();
      },
    });
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Users & quotas</CardTitle>
          <Button onClick={() => edit()}>
            <Plus />
            User
          </Button>
        </div>
        <p className="text-sm text-muted-foreground">
          Uploaded, copied and ZIP files count toward quota, including trash.
          Pre-existing files remain unassigned.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={users.error} />
        {users.data?.map((user) => (
          <div
            key={user.id}
            className="flex flex-wrap items-center gap-3 border-b py-4"
          >
            <div className="flex-1">
              <p className="font-semibold">
                {user.username}
                {user.id === me.id ? " (you)" : ""}
              </p>
              <p className="text-xs text-muted-foreground">
                {user.role} · {fmt(user.used)} /{" "}
                {user.quota ? fmt(user.quota) : "Unlimited"}
              </p>
              {!!user.quota && (
                <div className="mt-2 max-w-xs">
                  <Progress value={((user.used || 0) / user.quota) * 100} />
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={() => edit(user)}>
              Edit
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={user.id === me.id}
              onClick={() =>
                ask({
                  title: `Delete ${user.username}?`,
                  description:
                    "Remove owned files and trash first. This removes sessions, shares, favorites and permissions.",
                  fields: [],
                  danger: true,
                  submit: "Delete user",
                  run: async () => {
                    await api(`/api/users/${user.id}`, { method: "DELETE" });
                    await users.reload();
                    notify("User deleted");
                  },
                })
              }
            >
              Delete
            </Button>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
function PermissionPanel({
  ask,
  notify,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
}) {
  const rules = useResource<any[]>("/api/permissions"),
    users = useResource<User[]>("/api/users");
  const edit = (rule?: any) => {
    const options = (users.data || [])
      .filter((u) => u.role !== "admin")
      .map((u) => ({ value: String(u.id), label: u.username }));
    if (!options.length) {
      notify("Create a standard user first. Administrators have full access.");
      return;
    }
    ask({
      title: rule ? "Edit permission" : "Grant folder access",
      description:
        "Enter an existing relative folder; blank means root. The nearest rule applies to descendants. Deny overrides an inherited grant.",
      fields: [
        {
          key: "userId",
          label: "User",
          value: String(rule?.userId || options[0].value),
          options,
          disabled: !!rule,
        },
        {
          key: "path",
          label: "Folder (blank = root)",
          value: rule?.path || "",
          disabled: !!rule,
        },
        {
          key: "access",
          label: "Access",
          value: rule?.access || "read",
          options: [
            { value: "read", label: "Read only" },
            { value: "write", label: "Read & write" },
            { value: "none", label: "Deny" },
          ],
        },
      ],
      submit: "Save permission",
      run: async (v) => {
        await api("/api/permissions", {
          method: "POST",
          body: JSON.stringify({
            userId: Number(v.userId),
            path: v.path,
            access: v.access,
          }),
        });
        await rules.reload();
        notify("Permission saved");
      },
    });
  };
  return (
    <Card>
      <CardHeader>
        <div className="flex justify-between gap-2">
          <CardTitle>Folder permissions</CardTitle>
          <Button disabled={!users.data} onClick={() => edit()}>
            <Shield />
            Grant access
          </Button>
        </div>
        <p className="text-sm text-muted-foreground">
          New users have write access to their home folder. Granted folder ancestors are visible for
          navigation. Recursive operations respect nested rules.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={rules.error || users.error} />
        {rules.data?.map((rule) => (
          <div
            key={`${rule.userId}:${rule.path}`}
            className="flex flex-wrap items-center gap-3 border-b py-4"
          >
            <div className="flex-1">
              <b>{rule.username}</b>
              <p className="text-sm text-muted-foreground">
                /{rule.path} · {rule.access}
              </p>
            </div>
            <Button size="sm" variant="outline" onClick={() => edit(rule)}>
              Edit
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() =>
                ask({
                  title: "Remove rule?",
                  description:
                    "Access follows the nearest parent rule. Without a parent rule, access is denied.",
                  fields: [],
                  submit: "Remove rule",
                  run: async () => {
                    await api("/api/permissions", {
                      method: "POST",
                      body: JSON.stringify({
                        userId: rule.userId,
                        path: rule.path,
                        access: "inherit",
                      }),
                    });
                    await rules.reload();
                  },
                })
              }
            >
              Remove
            </Button>
          </div>
        ))}
        {rules.data?.length === 0 && <Empty text="No folder grants yet." />}
      </CardContent>
    </Card>
  );
}
function SystemPanel({ sys, network, trafficSamples, networkError }: { sys: any; network: any; trafficSamples: TrafficSample[]; networkError: string }) {
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>System information</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2">
          {sys &&
            [
              ["Hostname", sys.hostname],
              ["Operating system", sys.os],
              ["Kernel", sys.kernel],
              ["CPU", `${sys.cpu.toFixed(1)}%`],
              ["Memory", `${fmt(sys.memoryUsed)} / ${fmt(sys.memoryTotal)}`],
              ["Uptime", `${Math.floor(sys.uptime / 3600)} hours`],
            ].map(([name, value]) => (
              <div key={name} className="rounded-lg bg-accent p-4">
                <p className="text-xs text-muted-foreground">{name}</p>
                <p className="font-semibold">{value}</p>
              </div>
            ))}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Network speed</CardTitle>
          <p className="text-sm text-muted-foreground">
            Actual traffic sampled every second; loopback excluded.
          </p>
        </CardHeader>
        <CardContent>
          <NetworkTrafficGraph samples={trafficSamples} unavailable={!!networkError || network?.available === false} />
          {network?.interfaces.map((item: any) => (
            <div
              key={item.name}
              className="flex flex-wrap justify-between gap-3 border-b py-4"
            >
              <b>{item.name}</b>
              <span className="text-sm">
                ↓ {fmt(item.rxBytesPerSecond)}/s · ↑{" "}
                {fmt(item.txBytesPerSecond)}/s
              </span>
              <span className="text-xs text-muted-foreground">
                Received {fmt(item.received)} · Sent {fmt(item.sent)}
              </span>
            </div>
          ))}
          {network && !network.available && (
            <Empty text="Network counters unavailable on this host." />
          )}
        </CardContent>
      </Card>
    </div>
  );
}
function HardwarePanel() {
  const hardware = useResource<any>("/api/hardware", 60000);
  return (
    <Card>
      <CardHeader>
        <div className="flex justify-between">
          <CardTitle>SMART disk health & temperature</CardTitle>
          <Button
            variant="outline"
            size="icon"
            aria-label="Refresh disk health"
            onClick={() => void hardware.reload("/api/hardware?refresh=true")}
          >
            <RefreshCw />
          </Button>
        </div>
        <p className="text-sm text-muted-foreground">
          Readings cached for one minute.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={hardware.error} />
        {!hardware.data && !hardware.error && (
          <Empty text="Reading disk health…" />
        )}
        <p className="mb-4 text-sm text-muted-foreground">
          {hardware.data?.message}
        </p>
        {hardware.data?.accessDenied && (
          <div className="mb-5 rounded-xl border border-primary/20 bg-primary/5 p-4 text-sm">
            <p className="font-semibold">Enable read-only SMART access</p>
            <p className="mt-2 text-muted-foreground">
              Run this once in a terminal from the project folder. It grants
              only fixed SMART read commands for the four configured disks.
            </p>
            <code className="mt-3 block overflow-x-auto rounded-lg bg-background p-3 text-xs">
              sudo ./scripts/install-smart-access.sh{" "}
              {hardware.data.serviceUser || "YOUR_API_USERNAME"}
            </code>
            <p className="mt-3 text-muted-foreground">
              Set <code>SMART_USE_SUDO=true</code> in <code>.env</code>, restart
              the backend, then click refresh. The API can continue running as
              your normal user.
            </p>
          </div>
        )}
        <div className="grid gap-4 lg:grid-cols-2">
          {hardware.data?.disks.map((disk: any) => (
            <div key={disk.device} className="rounded-lg border p-4">
              <div className="flex items-center justify-between gap-3">
                <div className="flex min-w-0 items-center gap-3">
                  <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
                    <HardDrive className="size-5" aria-hidden="true" />
                  </span>
                  <div className="min-w-0">
                    <b className="block break-words">{disk.model}</b>
                    <p className="break-all text-xs text-muted-foreground">
                      {disk.device} · {disk.type}
                    </p>
                  </div>
                </div>
                <span
                  className={cn(
                    "rounded px-2 py-1 text-xs",
                    disk.health === "passed"
                      ? "bg-primary/10 text-primary"
                      : disk.health === "failed"
                        ? "bg-destructive/10 text-destructive"
                        : "bg-secondary",
                  )}
                >
                  {disk.health}
                </span>
              </div>
              <p className="mt-4 text-3xl font-semibold">
                {disk.temperature == null
                  ? "Unavailable"
                  : `${disk.temperature} °C`}
              </p>
              <p className="mt-2 text-xs text-muted-foreground">
                Power-on time:{" "}
                {disk.powerOnHours == null
                  ? "Unavailable"
                  : `${disk.powerOnHours} hours`}
                {disk.wearPercent != null && ` · Wear: ${disk.wearPercent}%`}
              </p>
              {disk.message && (
                <p className="mt-2 text-sm text-muted-foreground">
                  {disk.message}
                </p>
              )}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
function ActivityPanel() {
  const activity = useResource<any[]>("/api/activity", 10000);
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent activity</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={activity.error} />
        {activity.data?.map((item, i) => (
          <div
            key={i}
            className="grid gap-1 border-b py-3 text-sm sm:grid-cols-[110px_1fr_180px]"
          >
            <b>{item.user}</b>
            <div>
              {item.action}
              <p className="break-all text-xs text-muted-foreground">
                {item.path}
              </p>
            </div>
            <span className="text-xs text-muted-foreground">
              {new Date(item.time).toLocaleString()}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
function FavoritesPanel({
  openPath,
  notify,
}: {
  openPath: (p: string) => void;
  notify: (s: string) => void;
}) {
  const favorites = useResource<any[]>("/api/favorites");
  return (
    <Card>
      <CardHeader>
        <CardTitle>Favorites</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={favorites.error} />
        {favorites.data?.map((item) => (
          <div key={item.id} className="flex items-center gap-3 border-b py-3">
            <Star className="size-4 text-amber-500" />
            <button
              className="flex-1 break-all text-left text-sm"
              title="Double-click to open"
              onDoubleClick={() => openPath(item.path)}
              onClick={(event) => {
                if (event.detail === 0) openPath(item.path);
              }}
            >
              {item.path}
            </button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                void api("/api/favorites", {
                  method: "POST",
                  body: JSON.stringify({ path: item.path }),
                })
                  .then(() => favorites.reload())
                  .catch((e) => notify(message(e)))
              }
            >
              Remove
            </Button>
          </div>
        ))}
        {favorites.data?.length === 0 && <Empty text="No favorites yet." />}
      </CardContent>
    </Card>
  );
}
function SharesPanel({
  ask,
  notify,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
}) {
  const shares = useResource<any[]>("/api/shares");
  return (
    <Card>
      <CardHeader>
        <CardTitle>Shared links</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={shares.error} />
        {shares.data?.map((item) => (
          <div
            key={item.id}
            className="flex flex-wrap items-center gap-2 border-b py-4"
          >
            <div className="min-w-0 flex-1">
              <p className="break-all text-sm font-semibold">{item.paths?.length > 1 ? `${item.paths.length} shared items` : item.path || "Storage root"}</p>
              {item.paths?.length > 1 && <p className="break-all text-xs text-muted-foreground">{item.paths.join(", ")}</p>}
              {item.passwordProtected && <p className="text-xs text-primary">Password protected</p>}
              <p className="text-xs text-muted-foreground">
                {item.expiresAt
                  ? `Expires ${new Date(item.expiresAt).toLocaleString()}`
                  : "No expiry"}
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                void navigator.clipboard
                  .writeText(shareURL(item.token))
                  .then(() => notify("Share link copied"))
                  .catch(() => notify(`Share link: ${shareURL(item.token)}`))
              }
            >
              Copy link
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                ask({
                  title: "Revoke link?",
                  description: "Anyone using this link will lose access.",
                  fields: [],
                  danger: true,
                  submit: "Revoke",
                  run: async () => {
                    await api(`/api/shares/${item.id}`, { method: "DELETE" });
                    await shares.reload();
                  },
                })
              }
            >
              Revoke
            </Button>
          </div>
        ))}
        {shares.data?.length === 0 && <Empty text="No shared links." />}
      </CardContent>
    </Card>
  );
}
function TrashPanel({
  ask,
  notify,
  onChanged,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
  onChanged: () => void;
}) {
  const trash = useResource<any[]>("/api/trash");
  return (
    <Card>
      <CardHeader>
        <CardTitle>Trash</CardTitle>
        <p className="text-sm text-muted-foreground">
          Trash counts toward quota. Restore requires write access to the
          original folder.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={trash.error} />
        {trash.data?.map((item) => (
          <div
            key={item.id}
            className="flex flex-wrap items-center gap-3 border-b py-4"
          >
            <Trash2 className="size-4 text-muted-foreground" />
            <div className="min-w-0 flex-1">
              <b className="text-sm">{item.name}</b>
              <p className="break-all text-xs text-muted-foreground">
                {item.path}
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                void api("/api/trash/restore", {
                  method: "POST",
                  body: JSON.stringify({ id: item.id }),
                })
                  .then(async () => {
                    await trash.reload();
                    onChanged();
                    notify("Item restored");
                  })
                  .catch((e) => notify(message(e)))
              }
            >
              Restore
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                ask({
                  title: "Delete permanently?",
                  description: `${item.path} will be permanently removed, freeing quota for its owner.`,
                  fields: [],
                  danger: true,
                  submit: "Delete forever",
                  run: async () => {
                    await api(`/api/trash?id=${item.id}`, { method: "DELETE" });
                    await trash.reload();
                    onChanged();
                  },
                })
              }
            >
              Delete forever
            </Button>
          </div>
        ))}
        {trash.data?.length === 0 && <Empty text="Trash is empty." />}
      </CardContent>
    </Card>
  );
}
function Preview({ item, close }: { item: Item; close: () => void }) {
  const ext = item.name.split(".").pop()?.toLowerCase() || "",
    url = `${API}/api/files/preview?path=${encodeURIComponent(item.path)}`;
  let content: ReactNode = (
    <Empty text="Preview unavailable. Download this file to open it." />
  );
  if (["jpg", "jpeg", "png", "gif", "webp", "svg"].includes(ext))
    content = (
      <img
        alt={item.name}
        src={url}
        className="mx-auto max-h-[65vh] max-w-full"
      />
    );
  else if (["mp4", "webm"].includes(ext))
    content = (
      <video src={url} controls className="mx-auto max-h-[65vh] w-full" />
    );
  else if (["mp3", "wav", "ogg", "m4a"].includes(ext))
    content = <audio src={url} controls className="w-full" />;
  else if (
    ["pdf", "txt", "md", "json", "log", "csv", "yaml", "yml"].includes(ext)
  )
    content = (
      <iframe
        title={`Preview of ${item.name}`}
        src={url}
        sandbox=""
        className="h-[65vh] w-full rounded border bg-white"
      />
    );
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      <DialogContent className="max-w-5xl">
        <DialogTitle className="pr-6 font-semibold">{item.name}</DialogTitle>
        <DialogDescription className="mb-4 break-all text-xs text-muted-foreground">
          {item.path}
        </DialogDescription>
        {content}
        <div className="mt-4">
          <Button asChild>
            <a
              href={`${API}/api/files/download?path=${encodeURIComponent(item.path)}`}
            >
              <Download />
              Download
            </a>
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

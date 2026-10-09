"use client";

export type TrafficSample = { at: number; rx: number; tx: number };

const rate = (bytes: number) => {
  if (bytes >= 1048576) return `${(bytes / 1048576).toFixed(1)} MiB/s`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KiB/s`;
  return `${Math.round(bytes)} B/s`;
};

export function NetworkTrafficGraph({ samples, unavailable = false }: {
  samples: TrafficSample[];
  unavailable?: boolean;
}) {
  const latest = samples.at(-1);
  const end = latest?.at ?? 0;
  const visible = samples.filter(sample => sample.at >= end - 60000);
  const peak = Math.max(1, ...visible.flatMap(sample => [sample.rx, sample.tx])) * 1.15;
  const x = (at: number) => 4 + ((at - (end - 60000)) / 60000) * 312;
  const y = (value: number) => 76 - (value / peak) * 68;
  const path = (key: "rx" | "tx") => visible.map((sample, index) => {
    const gap = index > 0 && sample.at - visible[index - 1].at > 6500;
    return `${index === 0 || gap ? "M" : "L"}${x(sample.at).toFixed(1)},${y(sample[key]).toFixed(1)}`;
  }).join(" ");

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap justify-between gap-x-3 gap-y-1 text-[11px]">
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-primary" />↓ Download {latest && !unavailable ? rate(latest.rx) : "—"}</span>
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-emerald-500" />↑ Upload {latest && !unavailable ? rate(latest.tx) : "—"}</span>
      </div>
      <svg viewBox="0 0 320 84" className="h-24 w-full" role="img" aria-label="Network traffic over the last 60 seconds. Download is purple; upload is green.">
        {[8, 42, 76].map(row => <line key={row} x1="4" x2="316" y1={row} y2={row} stroke="var(--border)" strokeDasharray="3 4" />)}
        <path d={path("rx")} fill="none" stroke="var(--primary)" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" />
        <path d={path("tx")} fill="none" stroke="#10b981" strokeWidth="2" strokeDasharray="5 3" strokeLinejoin="round" strokeLinecap="round" />
        {visible.map(sample => <g key={sample.at}>
          <title>{new Date(sample.at).toLocaleTimeString()}: download {rate(sample.rx)}, upload {rate(sample.tx)}</title>
          <circle cx={x(sample.at)} cy={y(sample.rx)} r="2" fill="var(--primary)" />
          <circle cx={x(sample.at)} cy={y(sample.tx)} r="2" fill="#10b981" />
        </g>)}
      </svg>
      <div className="flex justify-between text-[10px] text-muted-foreground"><span>60s ago</span><span>{unavailable ? "Traffic unavailable" : !latest ? "Waiting for samples…" : `Peak scale ${rate(peak)}`}</span><span>Now</span></div>
    </div>
  );
}

"use client";

export type UsageSample = { at: number; cpu: number; memory: number };

export function UsageGraph({ samples, metric, unavailable }: {
  samples: UsageSample[];
  metric: "cpu" | "memory";
  unavailable: boolean;
}) {
  return <HistoryGraph samples={samples.map(sample => ({ at: sample.at, value: sample[metric] }))} label={metric === "cpu" ? "CPU" : "Memory"} unavailable={unavailable} color={metric === "cpu" ? "var(--primary)" : "#10b981"} />;
}

export type HistorySample = { at: number; value: number };
export function HistoryGraph({ samples, label, unavailable, maxValue = 100, formatValue = percent, color = "var(--primary)" }: {
  samples: HistorySample[];
  label: string;
  unavailable: boolean;
  maxValue?: number;
  formatValue?: (value: number) => string;
  color?: string;
}) {
  const end = samples.at(-1)?.at ?? 0;
  const visible = samples.filter(sample => sample.at >= end - 60000);
  const x = (at: number) => 4 + ((at - (end - 60000)) / 60000) * 312;
  const ceiling = Math.max(1, maxValue);
  const y = (value: number) => 76 - Math.min(1, Math.max(0, value / ceiling)) * 68;
  const path = visible.map((sample, index) => {
    const gap = index > 0 && sample.at - visible[index - 1].at > 11000;
    return `${index === 0 || gap ? "M" : "L"}${x(sample.at).toFixed(1)},${y(sample.value).toFixed(1)}`;
  }).join(" ");
  return (
    <div className="mt-3 space-y-2">
      <div className="flex justify-between text-[10px] text-muted-foreground"><span>{label} usage</span><span>0–{formatValue(ceiling)}</span></div>
      <svg viewBox="0 0 320 84" className="h-24 w-full" role="img" aria-label={`${label} usage over the last 60 seconds, on a scale from 0 to ${formatValue(ceiling)}.`}>
        {[8, 42, 76].map(row => <line key={row} x1="4" x2="316" y1={row} y2={row} stroke="var(--border)" strokeDasharray="3 4" />)}
        <path d={path} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" />
        {visible.map(sample => <circle key={sample.at} cx={x(sample.at)} cy={y(sample.value)} r="2" fill={color}>
          <title>{new Date(sample.at).toLocaleTimeString()}: {label} {formatValue(sample.value)}</title>
        </circle>)}
      </svg>
      <div className="flex justify-between text-[10px] text-muted-foreground"><span>60s ago</span><span>{unavailable ? "Data unavailable" : !visible.length ? "Waiting for samples…" : "Updated every 5s"}</span><span>Now</span></div>
    </div>
  );
}

const percent = (value: number) => `${value.toFixed(1)}%`;

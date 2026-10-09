"use client";

import { type ReactNode } from "react";
import { Activity, HardDrive, Server } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { NetworkTrafficGraph, type TrafficSample } from "@/modules/metrics/network-traffic-graph";
import { UsageGraph, HistoryGraph, type UsageSample, type HistorySample } from "@/modules/metrics/usage-graph";
import { type Quota } from "@/modules/shared/types";
import { fmt } from "@/modules/shared/format";

export function Metric({
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

export function Overview({
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

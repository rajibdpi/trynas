"use client";

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { NetworkTrafficGraph, type TrafficSample } from "@/modules/metrics/network-traffic-graph";
import { fmt } from "@/modules/shared/format";
import { Empty } from "@/modules/shared/feedback";

export function SystemPanel({ sys, network, trafficSamples, networkError }: { sys: any; network: any; trafficSamples: TrafficSample[]; networkError: string }) {
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

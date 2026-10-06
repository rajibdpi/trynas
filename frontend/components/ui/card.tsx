import * as React from "react";
import { cn } from "@/lib/utils";
export const Card = ({ className, ...p }: React.ComponentProps<"div">) => (
  <div
    className={cn(
      "surface-card rounded-2xl border bg-card text-card-foreground",
      className,
    )}
    {...p}
  />
);
export const CardHeader = ({
  className,
  ...p
}: React.ComponentProps<"div">) => (
  <div className={cn("flex flex-col gap-1.5 p-6", className)} {...p} />
);
export const CardTitle = ({ className, ...p }: React.ComponentProps<"div">) => (
  <div
    className={cn("font-semibold leading-snug tracking-tight", className)}
    {...p}
  />
);
export const CardContent = ({
  className,
  ...p
}: React.ComponentProps<"div">) => (
  <div className={cn("p-6 pt-0", className)} {...p} />
);

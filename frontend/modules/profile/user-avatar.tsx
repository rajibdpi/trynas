"use client";

import { useState } from "react";
import { UserRound } from "lucide-react";
import { API } from "@/modules/shared/api";
import { cn } from "@/modules/shared/utils";

export function UserAvatar({ avatarUrl, className }: { avatarUrl?: string; className?: string }) {
  const [failedSource, setFailedSource] = useState<string>();
  const source = avatarUrl ? API + avatarUrl : undefined;
  return <span className={cn("user-avatar shrink-0 overflow-hidden", className)}>
    {source && source !== failedSource ? <img src={source} alt="Profile image" className="h-full w-full object-cover" onError={() => setFailedSource(source)} /> : <UserRound aria-hidden="true" className="h-1/2 w-1/2" />}
  </span>;
}

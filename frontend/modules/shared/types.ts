import type { Eye } from "lucide-react";

export type User = {
 home?: string;
 rootAccessible?: boolean;
  id: number;
  username: string;
  role: string;
  quota?: number;
  used?: number;
};

export type Item = {
  name: string;
  path: string;
  dir: boolean;
  size: number;
  modified: string;
  favorite?: boolean;
  writable: boolean;
};

export type Quota = { used: number; limit: number; unlimited: boolean };

export type Field = {
  key: string;
  label: string;
  value: string;
  type?: string;
  options?: { value: string; label: string }[];
  required?: boolean;
  disabled?: boolean;
  visibleWhen?: { key: string; value: string };
};

export type Command = {
  title: string;
  description: string;
  fields: Field[];
  danger?: boolean;
  destinationFolders?: string[];
 destinationRoot?: string;
  submit: string;
  run: (values: Record<string, string>) => Promise<void>;
};

export type Action = {
  label: string;
  icon: typeof Eye;
  disabled?: boolean;
  danger?: boolean;
  run: () => void;
};

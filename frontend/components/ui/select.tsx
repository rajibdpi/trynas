"use client";

import * as React from "react";
import * as Menu from "@radix-ui/react-dropdown-menu";
import { Check, ChevronDown } from "lucide-react";
import { DropdownMenuContent } from "@/components/ui/dropdown-menu";
import { cn } from "@/modules/shared/utils";

type OptionProps = React.ComponentProps<"option">;
type SelectProps = Omit<React.ComponentProps<"button">, "value" | "onChange" | "children"> & {
  value: string;
  onValueChange: (value: string) => void;
  children: React.ReactNode;
};

export function Select({ value, onValueChange, children, className, disabled, ...props }: SelectProps) {
  const options = React.Children.toArray(children).filter(
    (child): child is React.ReactElement<OptionProps> => React.isValidElement<OptionProps>(child),
  );
  const optionValue = (option: React.ReactElement<OptionProps>) => String(option.props.value ?? option.props.children ?? "");
  const selected = options.find(option => optionValue(option) === value);
  return (
    <Menu.Root modal={false}>
      <Menu.Trigger asChild>
        <button {...props} type="button" disabled={disabled} className={cn(
          "flex min-h-[42px] w-full min-w-0 items-center justify-between gap-3 rounded-[10px] border border-border bg-card px-3 py-2 text-left text-sm font-medium shadow-sm transition-colors hover:border-primary/45 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-primary/20 focus-visible:border-primary disabled:cursor-not-allowed disabled:opacity-50 data-[state=open]:border-primary data-[state=open]:ring-3 data-[state=open]:ring-primary/20",
          className,
        )}>
          <span className="min-w-0 truncate">{selected?.props.children ?? "Choose an option"}</span>
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        </button>
      </Menu.Trigger>
      <DropdownMenuContent align="start" className="min-w-[var(--radix-dropdown-menu-trigger-width)]">
        <Menu.RadioGroup value={value} onValueChange={onValueChange}>
          {options.map(option => (
            <Menu.RadioItem key={optionValue(option)} value={optionValue(option)} disabled={option.props.disabled} className="flex min-h-10 cursor-pointer items-center justify-between gap-4 rounded-lg px-3 py-2 text-sm outline-none transition-colors data-[highlighted]:bg-accent data-[state=checked]:bg-primary/10 data-[state=checked]:font-medium data-[state=checked]:text-primary data-[disabled]:pointer-events-none data-[disabled]:opacity-40">
              <span className="min-w-0 break-words">{option.props.children}</span>
              <span className="size-4 shrink-0"><Menu.ItemIndicator><Check className="size-4" aria-hidden="true" /></Menu.ItemIndicator></span>
            </Menu.RadioItem>
          ))}
        </Menu.RadioGroup>
      </DropdownMenuContent>
    </Menu.Root>
  );
}

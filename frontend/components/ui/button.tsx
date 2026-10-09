import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/modules/shared/utils";
const variants = cva(
  "inline-flex items-center justify-center gap-2 rounded-lg text-sm font-medium transition-all focus-visible:outline-2 focus-visible:outline-primary active:scale-[.98] disabled:pointer-events-none disabled:opacity-50 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        secondary: "bg-secondary text-foreground hover:bg-secondary/80",
        outline: "border bg-background hover:bg-accent",
        ghost: "hover:bg-accent hover:text-accent-foreground",
        destructive: "bg-destructive text-white hover:bg-destructive/90",
      },
      size: {
        default: "h-10 px-4 py-2 [&_svg]:size-4",
        xs: "h-7 gap-1 px-2 text-xs [&_svg]:size-3",
        sm: "h-8 px-3 [&_svg]:size-4",
        md: "h-10 px-4 py-2 [&_svg]:size-4",
        lg: "h-11 px-6 text-base [&_svg]:size-5",
        xl: "h-12 px-8 text-lg [&_svg]:size-6",
        icon: "size-10 [&_svg]:size-4",
        "icon-xs": "size-7 [&_svg]:size-3",
        "icon-sm": "size-8 [&_svg]:size-4",
        "icon-md": "size-10 [&_svg]:size-4",
        "icon-lg": "size-11 [&_svg]:size-5",
        "icon-xl": "size-12 [&_svg]:size-6",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);
export function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof variants> & { asChild?: boolean }) {
  const C = asChild ? Slot : "button";
  return (
    <C className={cn(variants({ variant, size, className }))} {...props} />
  );
}

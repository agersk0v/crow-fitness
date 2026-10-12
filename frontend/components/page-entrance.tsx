"use client";

import { useEntranceAnimation } from "@/hooks/use-entrance-animation";

export function PageEntrance({
  children,
  className,
}: {
  children: React.ReactNode;
  className: string;
}) {
  const scope = useEntranceAnimation<HTMLElement>();
  return (
    <section ref={scope} className={className}>
      {children}
    </section>
  );
}

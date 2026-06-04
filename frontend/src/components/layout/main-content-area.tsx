"use client";

import { usePathname } from "next/navigation";
import {
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { PageTransitionLoading } from "@/components/layout/page-transition-loading";
import { cn } from "@/lib/utils";

const MIN_LOADING_MS = 320;

function normalizePath(path: string): string {
  const trimmed = path.replace(/\/$/, "");
  return trimmed || "/";
}

function isInternalNavigation(href: string, pathname: string): boolean {
  if (!href || href.startsWith("#")) return false;

  try {
    const url = new URL(href, window.location.origin);
    if (url.origin !== window.location.origin) return false;
    return normalizePath(url.pathname) !== normalizePath(pathname);
  } catch {
    return false;
  }
}

function beginNavigation(
  setPending: (value: boolean) => void,
  startedAtRef: RefObject<number | null>,
) {
  startedAtRef.current = Date.now();
  setPending(true);
}

export function MainContentArea({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [pending, setPending] = useState(false);
  const startedAtRef = useRef<number | null>(null);
  const pathnameRef = useRef(pathname);
  const prevPathnameRef = useRef(pathname);
  const finishTimeoutRef = useRef<number | null>(null);

  const clearFinishTimeout = () => {
    if (finishTimeoutRef.current !== null) {
      window.clearTimeout(finishTimeoutRef.current);
      finishTimeoutRef.current = null;
    }
  };

  const scheduleFinish = () => {
    clearFinishTimeout();
    const startedAt = startedAtRef.current ?? Date.now();
    const remaining = Math.max(0, MIN_LOADING_MS - (Date.now() - startedAt));

    finishTimeoutRef.current = window.setTimeout(() => {
      setPending(false);
      startedAtRef.current = null;
      finishTimeoutRef.current = null;
    }, remaining);
  };

  useEffect(() => {
    const handleClick = (event: MouseEvent) => {
      if (
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      ) {
        return;
      }

      const anchor = (event.target as Element | null)?.closest("a");
      if (!anchor || anchor.target === "_blank" || anchor.hasAttribute("download")) {
        return;
      }

      const href = anchor.getAttribute("href");
      if (!href || !isInternalNavigation(href, pathnameRef.current)) {
        return;
      }

      beginNavigation(setPending, startedAtRef);
    };

    const handlePopState = () => {
      beginNavigation(setPending, startedAtRef);
    };

    document.addEventListener("click", handleClick, true);
    window.addEventListener("popstate", handlePopState);

    return () => {
      document.removeEventListener("click", handleClick, true);
      window.removeEventListener("popstate", handlePopState);
      clearFinishTimeout();
    };
  }, []);

  useEffect(() => {
    const prev = normalizePath(prevPathnameRef.current);
    const next = normalizePath(pathname);
    pathnameRef.current = pathname;

    if (prev === next) return;

    prevPathnameRef.current = pathname;

    if (!pending) {
      beginNavigation(setPending, startedAtRef);
    }

    scheduleFinish();
    return clearFinishTimeout;
  }, [pathname, children, pending]);

  return (
    <div className="relative min-h-[calc(100dvh-3.5rem)]">
      <div
        className={cn(
          "transition-opacity duration-300 ease-out",
          pending ? "pointer-events-none opacity-0" : "animate-in fade-in duration-300",
        )}
      >
        {children}
      </div>
      {pending && <PageTransitionLoading />}
    </div>
  );
}

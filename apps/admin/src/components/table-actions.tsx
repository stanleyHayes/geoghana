"use client";

import type { ReactNode } from "react";
import { Tooltip } from "@ghanageo/ui";

export function TableActions({
  children,
  label = "Row actions",
}: {
  children: ReactNode;
  label?: string;
}) {
  return (
    <div className="admin-table-actions" role="group" aria-label={label}>
      {children}
    </div>
  );
}

export function TableActionLink({
  href,
  label,
  children,
  tone = "default",
}: {
  href: string;
  label: string;
  children: ReactNode;
  tone?: "default" | "danger";
}) {
  return (
    <Tooltip content={label}>
      <a
        className="admin-table-action"
        data-tone={tone}
        href={href}
        aria-label={label}
      >
        {children}
      </a>
    </Tooltip>
  );
}

export function TableActionButton({
  label,
  children,
  onClick,
  disabled,
  tone = "default",
}: {
  label: string;
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  tone?: "default" | "danger";
}) {
  return (
    <Tooltip content={label}>
      <button
        className="admin-table-action"
        data-tone={tone}
        type="button"
        aria-label={label}
        onClick={onClick}
        disabled={disabled}
      >
        {children}
      </button>
    </Tooltip>
  );
}

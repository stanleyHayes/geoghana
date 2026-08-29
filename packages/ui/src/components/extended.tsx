"use client";

import * as DialogPrimitive from "@radix-ui/react-dialog";
import * as DropdownPrimitive from "@radix-ui/react-dropdown-menu";
import * as PopoverPrimitive from "@radix-ui/react-popover";
import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { Check, ChevronDown, ChevronLeft, ChevronRight, Search, X } from "lucide-react";
import {
  createContext, forwardRef, useContext, useId, useMemo, useState,
  type HTMLAttributes, type ReactNode,
} from "react";
import { cn } from "../lib/utils";
import { Button, Input } from "./primitives";

export type TabItem = { value: string; label: string; content: ReactNode; disabled?: boolean };
export function Tabs({ items, value, defaultValue, onValueChange, ariaLabel = "Sections" }: { items: readonly TabItem[]; value?: string; defaultValue?: string; onValueChange?: (value: string) => void; ariaLabel?: string }) {
  const first = defaultValue ?? items.find((item) => !item.disabled)?.value ?? "";
  const [internal, setInternal] = useState(first);
  const active = value ?? internal;
  const select = (next: string) => { if (value === undefined) setInternal(next); onValueChange?.(next); };
  return <div className="gg-tabs"><div className="gg-tabs__list" role="tablist" aria-label={ariaLabel}>{items.map((item) => <button key={item.value} type="button" role="tab" aria-selected={active === item.value} aria-controls={`panel-${item.value}`} disabled={item.disabled} onClick={() => select(item.value)}>{item.label}</button>)}</div>{items.map((item) => active === item.value ? <div key={item.value} id={`panel-${item.value}`} role="tabpanel" tabIndex={0} className="gg-tabs__panel">{item.content}</div> : null)}</div>;
}

export function Dialog({ trigger, title, description, children, open, onOpenChange }: { trigger?: ReactNode; title: string; description?: string; children: ReactNode; open?: boolean; onOpenChange?: (open: boolean) => void }) {
  return <DialogPrimitive.Root {...(open === undefined ? {} : { open })} {...(onOpenChange ? { onOpenChange } : {})}>{trigger ? <DialogPrimitive.Trigger asChild>{trigger}</DialogPrimitive.Trigger> : null}<DialogPrimitive.Portal><DialogPrimitive.Overlay className="gg-overlay"/><DialogPrimitive.Content className="gg-dialog"><div className="gg-dialog__head"><div><DialogPrimitive.Title>{title}</DialogPrimitive.Title>{description ? <DialogPrimitive.Description>{description}</DialogPrimitive.Description> : null}</div><DialogPrimitive.Close className="gg-icon-button" aria-label="Close dialog" title="Close dialog"><X size={18}/></DialogPrimitive.Close></div>{children}</DialogPrimitive.Content></DialogPrimitive.Portal></DialogPrimitive.Root>;
}

export function Sheet({ trigger, title, description, children, side = "right" }: { trigger: ReactNode; title: string; description?: string; children: ReactNode; side?: "left" | "right" }) {
  return <DialogPrimitive.Root><DialogPrimitive.Trigger asChild>{trigger}</DialogPrimitive.Trigger><DialogPrimitive.Portal><DialogPrimitive.Overlay className="gg-overlay"/><DialogPrimitive.Content className="gg-sheet" data-side={side}><div className="gg-dialog__head"><div><DialogPrimitive.Title>{title}</DialogPrimitive.Title>{description ? <DialogPrimitive.Description>{description}</DialogPrimitive.Description> : null}</div><DialogPrimitive.Close className="gg-icon-button" aria-label="Close sheet" title="Close sheet"><X size={18}/></DialogPrimitive.Close></div>{children}</DialogPrimitive.Content></DialogPrimitive.Portal></DialogPrimitive.Root>;
}
export const Drawer = Sheet;

export function Popover({ trigger, children, align = "start" }: { trigger: ReactNode; children: ReactNode; align?: "start" | "center" | "end" }) {
  return <PopoverPrimitive.Root><PopoverPrimitive.Trigger asChild>{trigger}</PopoverPrimitive.Trigger><PopoverPrimitive.Portal><PopoverPrimitive.Content className="gg-popover" sideOffset={8} align={align}>{children}<PopoverPrimitive.Arrow className="gg-popover__arrow"/></PopoverPrimitive.Content></PopoverPrimitive.Portal></PopoverPrimitive.Root>;
}

export function Dropdown({ trigger, label, items }: { trigger: ReactNode; label?: string; items: readonly { label: string; hint?: string; onSelect?: () => void; danger?: boolean; disabled?: boolean }[] }) {
  return <DropdownPrimitive.Root><DropdownPrimitive.Trigger asChild>{trigger}</DropdownPrimitive.Trigger><DropdownPrimitive.Portal><DropdownPrimitive.Content className="gg-dropdown" sideOffset={8}>{label ? <DropdownPrimitive.Label className="gg-dropdown__label">{label}</DropdownPrimitive.Label> : null}{items.map((item, index) => <DropdownPrimitive.Item key={`${item.label}-${index}`} className="gg-dropdown__item" data-danger={item.danger ? "" : undefined} {...(item.disabled === undefined ? {} : { disabled: item.disabled })} {...(item.onSelect ? { onSelect: item.onSelect } : {})}><span>{item.label}{item.hint ? <small>{item.hint}</small> : null}</span></DropdownPrimitive.Item>)}</DropdownPrimitive.Content></DropdownPrimitive.Portal></DropdownPrimitive.Root>;
}

export function Tooltip({ children, content, side = "top" }: { children: ReactNode; content: ReactNode; side?: "top" | "right" | "bottom" | "left" }) {
  return <TooltipPrimitive.Provider delayDuration={400}><TooltipPrimitive.Root><TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger><TooltipPrimitive.Portal><TooltipPrimitive.Content className="gg-tooltip" side={side} sideOffset={6}>{content}<TooltipPrimitive.Arrow className="gg-tooltip__arrow"/></TooltipPrimitive.Content></TooltipPrimitive.Portal></TooltipPrimitive.Root></TooltipPrimitive.Provider>;
}

type ToastItem = { id: string; title: string; description?: string; tone?: "neutral" | "success" | "danger" };
const ToastContext = createContext<{ push: (toast: Omit<ToastItem, "id">) => void } | null>(null);
export function ToastProvider({ children }: { children: ReactNode }) { const [toasts, setToasts] = useState<ToastItem[]>([]); const push = (toast: Omit<ToastItem, "id">) => { const id = crypto.randomUUID(); setToasts((current) => [...current.slice(-2), { ...toast, id }]); window.setTimeout(() => setToasts((current) => current.filter((item) => item.id !== id)), 4500); }; return <ToastContext.Provider value={{ push }}>{children}<div className="gg-toasts" aria-live="polite" aria-atomic="false">{toasts.map((toast) => <div className="gg-toast" data-tone={toast.tone ?? "neutral"} key={toast.id}><span><strong>{toast.title}</strong>{toast.description ? <small>{toast.description}</small> : null}</span><button onClick={() => setToasts((current) => current.filter((item) => item.id !== toast.id))} aria-label="Dismiss notification" title="Dismiss notification"><X size={15}/></button></div>)}</div></ToastContext.Provider>; }
export function useToast() { const context = useContext(ToastContext); if (!context) throw new Error("useToast must be used inside ToastProvider"); return context; }

export const Table = forwardRef<HTMLTableElement, HTMLAttributes<HTMLTableElement>>(({ className, ...props }, ref) => <div className="gg-table-wrap"><table ref={ref} className={cn("gg-table", className)} {...props}/></div>);
Table.displayName = "Table";

export function Pagination({ page, pageCount, hasNext, onPageChange, label = "Pagination" }: { page: number; pageCount?: number; hasNext?: boolean; onPageChange: (page: number) => void; label?: string }) { const canAdvance = pageCount === undefined ? Boolean(hasNext) : page < pageCount; return <nav className="gg-pagination" aria-label={label}><Button size="icon" variant="ghost" disabled={page <= 1} onClick={() => onPageChange(page - 1)} aria-label="Previous page" title="Previous page"><ChevronLeft size={17}/></Button><span>Page <strong>{page}</strong>{pageCount === undefined ? null : <> of {pageCount}</>}</span><Button size="icon" variant="ghost" disabled={!canAdvance} onClick={() => onPageChange(page + 1)} aria-label="Next page" title="Next page"><ChevronRight size={17}/></Button></nav>; }

export function Breadcrumb({ items, label = "Breadcrumb" }: { items: readonly { label: string; href?: string }[]; label?: string }) { return <nav aria-label={label}><ol className="gg-breadcrumb">{items.map((item, index) => <li key={`${item.label}-${index}`}>{index ? <ChevronRight size={13} aria-hidden/> : null}{item.href && index < items.length - 1 ? <a href={item.href}>{item.label}</a> : <span aria-current={index === items.length - 1 ? "page" : undefined}>{item.label}</span>}</li>)}</ol></nav>; }
export function Avatar({ name, src, size = "md" }: { name: string; src?: string; size?: "sm" | "md" | "lg" }) { const initials = name.split(/\s+/).map((word) => word[0]).join("").slice(0, 2).toUpperCase(); return <span className="gg-avatar" data-size={size}>{src ? <img src={src} alt=""/> : <span aria-hidden>{initials}</span>}<span className="gg-sr-only">{name}</span></span>; }
export function Progress({ value, max = 100, label }: { value: number; max?: number; label: string }) { const bounded = Math.max(0, Math.min(value, max)); return <div className="gg-progress"><div className="gg-progress__labels"><span>{label}</span><span>{Math.round((bounded / max) * 100)}%</span></div><div role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={max} aria-valuenow={bounded}><span style={{ width: `${(bounded / max) * 100}%` }}/></div></div>; }
export function Alert({ title, children, tone = "neutral" }: { title: string; children?: ReactNode; tone?: "neutral" | "info" | "success" | "warning" | "danger" }) { return <div className="gg-alert" data-tone={tone} role={tone === "danger" ? "alert" : "status"}><strong>{title}</strong>{children ? <div>{children}</div> : null}</div>; }

export function Combobox({ options, value, onValueChange, placeholder = "Search…", label = "Choose an option", emptyText = "No results" }: { options: readonly { value: string; label: string; hint?: string }[]; value?: string; onValueChange?: (value: string) => void; placeholder?: string; label?: string; emptyText?: string }) { const [open, setOpen] = useState(false); const [query, setQuery] = useState(""); const listId = useId(); const filtered = useMemo(() => options.filter((option) => option.label.toLocaleLowerCase().includes(query.toLocaleLowerCase())), [options, query]); return <div className="gg-combobox"><div className="gg-combobox__input"><Search size={16} aria-hidden/><Input role="combobox" aria-label={label} aria-expanded={open} aria-controls={listId} autoComplete="off" value={query || options.find((option) => option.value === value)?.label || ""} placeholder={placeholder} onFocus={() => setOpen(true)} onChange={(event) => { setQuery(event.target.value); setOpen(true); }}/><ChevronDown size={15} aria-hidden/></div>{open ? <div id={listId} role="listbox" className="gg-combobox__list">{filtered.length ? filtered.map((option) => <button type="button" role="option" aria-selected={option.value === value} key={option.value} onMouseDown={(event) => event.preventDefault()} onClick={() => { onValueChange?.(option.value); setQuery(""); setOpen(false); }}><span>{option.label}{option.hint ? <small>{option.hint}</small> : null}</span>{option.value === value ? <Check size={15}/> : null}</button>) : <p>{emptyText}</p>}</div> : null}</div>; }

export function SlidingIndicator({ items, value, onValueChange, label = "View" }: { items: readonly { value: string; label: string }[]; value: string; onValueChange: (value: string) => void; label?: string }) { return <div className="gg-sliding" role="group" aria-label={label}>{items.map((item) => <button type="button" key={item.value} aria-pressed={item.value === value} onClick={() => onValueChange(item.value)}>{item.label}</button>)}</div>; }

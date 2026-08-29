import type { LucideIcon } from "lucide-react";

/** The six admin roles from agent_plan.md Appendix C. */
export type Role =
  | "SUPER_ADMIN"
  | "DATA_ADMIN"
  | "DATA_REVIEWER"
  | "DATA_CONTRIBUTOR"
  | "DEVELOPER_SUPPORT"
  | "SECURITY_AUDITOR";

export const ALL_ROLES: readonly Role[] = [
  "SUPER_ADMIN", "DATA_ADMIN", "DATA_REVIEWER",
  "DATA_CONTRIBUTOR", "DEVELOPER_SUPPORT", "SECURITY_AUDITOR",
];

export const ROLE_LABELS: Record<Role, string> = {
  SUPER_ADMIN: "Super Admin",
  DATA_ADMIN: "Data Admin",
  DATA_REVIEWER: "Data Reviewer",
  DATA_CONTRIBUTOR: "Data Contributor",
  DEVELOPER_SUPPORT: "Developer Support",
  SECURITY_AUDITOR: "Security / Auditor",
};

export interface NavItem {
  id: string;
  label: string;
  href: string;
  icon: LucideIcon;
  roles: readonly Role[];
  /** Live count. Subscribed, never hardcoded. */
  badge?: number;
  exact?: boolean;
}

export interface NavGroup {
  id: string;
  /** null renders an unlabelled pinned group at the top of the rail. */
  label: string | null;
  icon: LucideIcon;
  roles: readonly Role[];
  defaultOpen: boolean;
  items: NavItem[];
}

/** Three-layer role gating: filter groups, filter items, drop emptied groups.
 *  An item a role cannot read is hidden entirely — a nav entry must never
 *  dead-end in a 403. */
export function filterNav(groups: readonly NavGroup[], role: Role): NavGroup[] {
  return groups
    .filter((g) => g.roles.includes(role))
    .map((g) => ({ ...g, items: g.items.filter((i) => i.roles.includes(role)) }))
    .filter((g) => g.items.length > 0);
}

/** Group headers show a roll-up badge: the sum of their children. */
export function groupBadge(g: NavGroup): number {
  return g.items.reduce((sum, i) => sum + (i.badge ?? 0), 0);
}

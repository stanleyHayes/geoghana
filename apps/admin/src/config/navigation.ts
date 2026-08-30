/**
 * The admin navigation registry — the single source of truth for the rail.
 *
 * OWNED CENTRALLY by lane L11 (agent_plan.md section 8). Other lanes request an
 * entry via a coordination note rather than editing this file concurrently.
 *
 * The IA is derived from Build Specification section 17, plus what sections 4,
 * 12, 13, 18 and 20 imply. See DESIGN_SYSTEM.md 14.5.
 */
import {
  Activity, AppWindow, Building2, ClipboardCheck, Clock, Compass, Database,
  FileClock, FileDiff, Globe, HeartPulse, KeyRound, Landmark,
  LayoutDashboard, Layers, Map, MapPin, MailWarning,
  Package, Paperclip, PencilLine, Replace, Rocket,
  Route, ScrollText, Shield,
  Sliders, Spline, Tags, Target, Users, Workflow, ListChecks,
  CirclePlay,
} from "lucide-react";
import type { NavGroup, Role } from "@ghanageo/ui";

const ALL: readonly Role[] = [
  "SUPER_ADMIN", "DATA_ADMIN", "DATA_REVIEWER",
  "DATA_CONTRIBUTOR", "DEVELOPER_SUPPORT", "SECURITY_AUDITOR",
];
const DATA: readonly Role[] = ["SUPER_ADMIN", "DATA_ADMIN", "DATA_REVIEWER", "DATA_CONTRIBUTOR"];
const STEWARD: readonly Role[] = ["SUPER_ADMIN", "DATA_ADMIN", "DATA_REVIEWER"];
const ADMIN_ONLY: readonly Role[] = ["SUPER_ADMIN", "DATA_ADMIN"];
const SUPPORT: readonly Role[] = ["SUPER_ADMIN", "DEVELOPER_SUPPORT", "SECURITY_AUDITOR"];
const SECURITY: readonly Role[] = ["SUPER_ADMIN", "SECURITY_AUDITOR"];

export const NAVIGATION: NavGroup[] = [
  {
    id: "overview", label: null, icon: LayoutDashboard, roles: ALL, defaultOpen: true,
    items: [
      { id: "home", label: "Home", href: "/", icon: LayoutDashboard, roles: ALL, exact: true },
      { id: "activity", label: "Activity Feed", href: "/activity", icon: Activity, roles: ALL },
    ],
  },
  {
    id: "explore", label: "Explore", icon: Compass, roles: ALL, defaultOpen: true,
    items: [
      { id: "explorer", label: "Location Explorer", href: "/explorer", icon: Map, roles: ALL },
      { id: "geometry", label: "Geometry Workbench", href: "/explorer/geometry", icon: Spline, roles: STEWARD },
    ],
  },
  {
    id: "geography", label: "Geography", icon: Globe, roles: ALL, defaultOpen: true,
    items: [
      { id: "regions", label: "Regions", href: "/geography/regions", icon: Map, roles: ALL },
      { id: "districts", label: "Districts / MMDAs", href: "/geography/districts", icon: Landmark, roles: ALL },
      { id: "places", label: "Places / Localities", href: "/geography/places", icon: MapPin, roles: ALL },
      { id: "aliases", label: "Aliases & Name Variants", href: "/geography/aliases", icon: Tags, roles: ALL },
      { id: "roads", label: "Roads", href: "/geography/roads", icon: Route, roles: ALL },
      { id: "pois", label: "Points of Interest", href: "/geography/pois", icon: Building2, roles: ALL },
      { id: "redirects", label: "Merges & Redirects", href: "/geography/redirects", icon: Replace, roles: STEWARD },
    ],
  },
  {
    id: "ingest", label: "Ingestion & Sources", icon: Workflow, roles: DATA.concat("SECURITY_AUDITOR"), defaultOpen: false,
    items: [
      { id: "pipeline", label: "Pipeline Overview", href: "/ingest", icon: Workflow, roles: ALL },
      { id: "runs", label: "Import Runs", href: "/ingest/runs", icon: CirclePlay, roles: ALL },
      { id: "duplicates", label: "Duplicate Candidates", href: "/ingest/duplicates", icon: Layers, roles: STEWARD },
    ],
  },
  {
    id: "review", label: "Review & Change Control", icon: ClipboardCheck, roles: DATA.concat("SECURITY_AUDITOR"), defaultOpen: false,
    items: [
      { id: "queue", label: "Review Queue", href: "/review", icon: ClipboardCheck, roles: DATA },
      { id: "change-requests", label: "Change Requests", href: "/review/change-requests", icon: FileDiff, roles: DATA },
      { id: "submissions", label: "Community Submissions", href: "/review/submissions", icon: Users, roles: STEWARD },
      { id: "drafts", label: "My Drafts", href: "/review/drafts", icon: PencilLine, roles: DATA },
      { id: "evidence", label: "Evidence Locker", href: "/review/evidence", icon: Paperclip, roles: STEWARD },
    ],
  },
  {
    id: "releases", label: "Releases & Versioning", icon: Package, roles: DATA.concat("SECURITY_AUDITOR"), defaultOpen: false,
    items: [
      { id: "versions", label: "Dataset Versions", href: "/releases/versions", icon: Package, roles: ALL },
      { id: "release-pipeline", label: "Release Pipeline", href: "/releases/pipeline", icon: Rocket, roles: STEWARD },
      { id: "validation", label: "Validation Gates", href: "/releases/validation", icon: ListChecks, roles: ALL },
      { id: "changelog", label: "Changelog Composer", href: "/releases/changelog", icon: ScrollText, roles: ADMIN_ONLY },
      { id: "exports", label: "Exports & Downloads", href: "/releases/exports", icon: Package, roles: ALL },
    ],
  },
  {
    id: "developers", label: "Developers & Access", icon: KeyRound, roles: SUPPORT, defaultOpen: false,
    items: [
      { id: "orgs", label: "Organizations", href: "/developers/orgs", icon: Building2, roles: SUPPORT },
      { id: "dev-users", label: "Developer Users", href: "/developers/users", icon: Users, roles: SUPPORT },
      { id: "apps", label: "Applications", href: "/developers/apps", icon: AppWindow, roles: SUPPORT },
      { id: "keys", label: "API Keys", href: "/developers/keys", icon: KeyRound, roles: SUPPORT },
      { id: "plans", label: "Rate-Limit Plans", href: "/developers/plans", icon: Sliders, roles: ["SUPER_ADMIN"] },
      { id: "logs", label: "Request Logs", href: "/developers/logs", icon: FileClock, roles: SUPPORT },
    ],
  },
  {
    id: "ops", label: "Platform Health", icon: HeartPulse, roles: ALL, defaultOpen: false,
    items: [
      { id: "health", label: "Service Health", href: "/ops/health", icon: HeartPulse, roles: ALL },
      { id: "queues", label: "Queues & Workers", href: "/ops/queues", icon: Layers, roles: ADMIN_ONLY.concat("DEVELOPER_SUPPORT", "SECURITY_AUDITOR") },
      { id: "dlq", label: "Outbox & Dead Letters", href: "/ops/dlq", icon: MailWarning, roles: ADMIN_ONLY },
      { id: "freshness", label: "ETL Freshness", href: "/ops/freshness", icon: Clock, roles: ALL },
      { id: "slo", label: "SLOs & Error Budgets", href: "/ops/slo", icon: Target, roles: SUPPORT.concat("DATA_ADMIN") },
      { id: "performance", label: "Latency & Traffic", href: "/ops/performance", icon: Activity, roles: SUPPORT.concat("DATA_ADMIN") },
      { id: "database", label: "Database", href: "/ops/database", icon: Database, roles: ["SUPER_ADMIN", "SECURITY_AUDITOR"] },
    ],
  },
  {
    id: "security", label: "Security & Audit", icon: Shield, roles: SECURITY.concat("DATA_ADMIN", "DEVELOPER_SUPPORT"), defaultOpen: false,
    items: [
      { id: "audit", label: "Audit Log", href: "/security/audit", icon: FileClock, roles: SECURITY.concat("DATA_ADMIN") },
    ],
  },
];

"use client";

import { Badge, Card, EmptyState, ThemePicker } from "@ghanageo/ui";
import { Activity, Database, PackageCheck } from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import { getAdminDashboard } from "@/lib/admin-api";

export default function AdminHome() {
  const state = useApi((signal) => getAdminDashboard(signal), []);
  return (
    <>
      <PageHeader
        title="Ghana’s location data, as infrastructure"
        lede="Authoritative counts, release state and recent privileged activity from the protected administration API."
      />
      <AsyncState state={state} empty="No dashboard projection was returned.">
        {(dashboard) => (
          <>
            <div className="gg-auto-grid admin-overview-grid">
              {(
                [
                  ["Regions", dashboard.counts.regions],
                  ["Districts / MMDAs", dashboard.counts.districts],
                  ["Places", dashboard.counts.places],
                  ["Roads", dashboard.counts.roads],
                  ["Points of interest", dashboard.counts.pois],
                  ["Audit entries", dashboard.counts.auditEntries],
                ] as const
              ).map(([label, value]) => (
                <Card key={label}>
                  <p className="admin-metric-label">{label}</p>
                  <p className="admin-metric-value">{value.toLocaleString()}</p>
                  <Badge tone="canonical">Canonical count</Badge>
                </Card>
              ))}
            </div>
            <div className="admin-honesty-grid">
              <Card>
                <PackageCheck size={20} aria-hidden />
                <div>
                  <h2>Current release</h2>
                  <p>
                    {dashboard.currentRelease
                      ? `${dashboard.currentRelease.version} · ${dashboard.currentRelease.status.toLowerCase()}`
                      : "No current published release is recorded."}
                  </p>
                  {dashboard.currentRelease ? (
                    <a href="/releases/versions">View release lifecycle</a>
                  ) : null}
                </div>
                <Badge
                  tone={dashboard.currentRelease ? "canonical" : "neutral"}
                >
                  {dashboard.currentRelease ? "Recorded" : "Unavailable"}
                </Badge>
              </Card>
              <Card>
                <Database size={20} aria-hidden />
                <div>
                  <h2>Outbox</h2>
                  <p>
                    {dashboard.counts.pendingOutbox.toLocaleString()} pending ·{" "}
                    {dashboard.counts.deadOutbox.toLocaleString()} dead
                  </p>
                  <a href="/ops/queues">Inspect queue health</a>
                </div>
                <Badge
                  tone={dashboard.counts.deadOutbox ? "danger" : "canonical"}
                >
                  {dashboard.counts.deadOutbox ? "Needs attention" : "Clear"}
                </Badge>
              </Card>
            </div>
            <section
              className="admin-activity"
              aria-labelledby="recent-activity-title"
            >
              <div className="admin-section-heading">
                <Activity size={18} aria-hidden />
                <h2 id="recent-activity-title">Recent privileged activity</h2>
                <a href="/security/audit">Open audit log</a>
              </div>
              {dashboard.recentActivity.length ? (
                <ol>
                  {dashboard.recentActivity.map((entry) => (
                    <li key={entry.id}>
                      <span
                        className={`admin-status-dot admin-status-dot--${entry.outcome}`}
                      />
                      <div>
                        <strong>{entry.action}</strong>
                        <p>
                          {entry.actor.label ||
                            entry.actor.id ||
                            entry.actor.kind}{" "}
                          ·{" "}
                          {entry.target.label ||
                            entry.target.id ||
                            entry.target.kind}
                        </p>
                      </div>
                      <time dateTime={entry.at}>
                        {new Date(entry.at).toLocaleString()}
                      </time>
                    </li>
                  ))}
                </ol>
              ) : (
                <EmptyState
                  title="No recent activity"
                  description="Privileged operator actions will appear here after they are recorded in the audit trail."
                />
              )}
            </section>
          </>
        )}
      </AsyncState>
      <h2 className="admin-section-title">Appearance</h2>
      <ThemePicker />
    </>
  );
}

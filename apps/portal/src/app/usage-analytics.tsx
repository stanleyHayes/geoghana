"use client";

import {
  Activity,
  AlertTriangle,
  Clock3,
  Gauge,
  RefreshCw,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { portalRequest } from "./account-workspace";

type Breakdown = {
  label: string;
  requests: number;
  errors: number;
  quotaCost: number;
  avgLatencyMs: number;
};
type Summary = {
  since: string;
  requests: number;
  errors: number;
  quotaCost: number;
  avgLatencyMs: number;
  byProtocol: Breakdown[];
  byEndpoint: Breakdown[];
  byGeography: Breakdown[];
};
type RequestEvent = {
  requestId: string;
  protocol: string;
  operation: string;
  status: string;
  success: boolean;
  latencyMs: number;
  quotaCost: number;
  quotaLimit: number;
  quotaRemaining: number;
  geography: string;
  at: string;
};
type RequestPage = {
  data: RequestEvent[];
  pagination: { nextBefore: string | null; limit: number };
};

const emptySummary: Summary = {
  since: "",
  requests: 0,
  errors: 0,
  quotaCost: 0,
  avgLatencyMs: 0,
  byProtocol: [],
  byEndpoint: [],
  byGeography: [],
};

export function UsageAnalytics({
  organizationId,
  applicationId,
}: {
  organizationId: string;
  applicationId: string;
}) {
  const [summary, setSummary] = useState(emptySummary);
  const [requests, setRequests] = useState<RequestEvent[]>([]);
  const [nextBefore, setNextBefore] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");

  const base = `/developer/organizations/${organizationId}/applications/${applicationId}`;
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [summaryResponse, requestResponse] = await Promise.all([
        portalRequest<{ data: Summary }>(`${base}/usage?days=30`),
        portalRequest<RequestPage>(`${base}/requests?limit=25`),
      ]);
      setSummary(summaryResponse.data);
      setRequests(requestResponse.data);
      setNextBefore(requestResponse.pagination.nextBefore);
    } catch (caught) {
      setError((caught as Error).message);
    } finally {
      setLoading(false);
    }
  }, [base]);

  useEffect(() => {
    void load();
  }, [load]);

  async function loadOlder() {
    if (!nextBefore) return;
    setLoadingMore(true);
    setError("");
    try {
      const response = await portalRequest<RequestPage>(
        `${base}/requests?limit=25&before=${encodeURIComponent(nextBefore)}`,
      );
      setRequests((current) => [...current, ...response.data]);
      setNextBefore(response.pagination.nextBefore);
    } catch (caught) {
      setError((caught as Error).message);
    } finally {
      setLoadingMore(false);
    }
  }

  return (
    <section className="usage-workspace" aria-labelledby="usage-heading">
      <div className="portal-section-head">
        <div>
          <p>Live account telemetry · 30 days</p>
          <h3 id="usage-heading">Usage and request logs</h3>
        </div>
        <button onClick={() => void load()} disabled={loading}>
          <RefreshCw className={loading ? "spin" : ""} size={15} /> Refresh
        </button>
      </div>
      {error ? (
        <p className="account-message">
          <AlertTriangle size={15} /> {error}
        </p>
      ) : null}
      {loading ? (
        <UsageSkeleton />
      ) : (
        <>
          <div className="usage-metrics">
            <Metric
              icon={<Activity size={17} />}
              label="Requests"
              value={summary.requests.toLocaleString()}
            />
            <Metric
              icon={<Gauge size={17} />}
              label="Quota units"
              value={summary.quotaCost.toLocaleString()}
            />
            <Metric
              icon={<Clock3 size={17} />}
              label="Average latency"
              value={`${Math.round(summary.avgLatencyMs)} ms`}
            />
            <Metric
              icon={<AlertTriangle size={17} />}
              label="Errors"
              value={summary.errors.toLocaleString()}
            />
          </div>
          {summary.requests ? (
            <div className="usage-charts">
              <UsageChart
                title="Requests by protocol"
                data={summary.byProtocol}
              />
              <UsageChart
                title="Most-used endpoints"
                data={summary.byEndpoint}
              />
              <UsageChart
                title="Geographic filters"
                data={summary.byGeography}
                empty="No region or district filters recorded yet."
              />
            </div>
          ) : (
            <div className="usage-empty">
              <Activity size={20} />
              <div>
                <strong>No attributed requests yet</strong>
                <p>
                  Use one of this application’s API keys and its telemetry will
                  appear here.
                </p>
              </div>
            </div>
          )}
          <div className="request-log">
            <div className="request-log__heading">
              <h4>Recent requests</h4>
              <span>Authorization and secrets are never stored.</span>
            </div>
            {requests.length ? (
              <div
                className="request-log__table"
                role="table"
                aria-label="Application request logs"
                tabIndex={0}
              >
                <div role="row" className="request-log__header">
                  <span>Time</span>
                  <span>Request</span>
                  <span>Protocol</span>
                  <span>Status</span>
                  <span>Latency</span>
                  <span>Quota</span>
                </div>
                {requests.map((request) => (
                  <div role="row" key={request.requestId}>
                    <time dateTime={request.at}>
                      {new Date(request.at).toLocaleString()}
                    </time>
                    <span>
                      <strong>{request.operation}</strong>
                      <code>{request.requestId}</code>
                      {request.geography ? (
                        <small>{request.geography}</small>
                      ) : null}
                    </span>
                    <span className="request-protocol">{request.protocol}</span>
                    <span
                      className={
                        request.success ? "request-ok" : "request-error"
                      }
                    >
                      {request.status}
                    </span>
                    <span>{request.latencyMs} ms</span>
                    <span>
                      {request.quotaCost} · {request.quotaRemaining} left
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="request-log__empty">
                No requests in the current retention window.
              </p>
            )}
            {nextBefore ? (
              <button
                className="portal-secondary request-log__more"
                onClick={() => void loadOlder()}
                disabled={loadingMore}
              >
                {loadingMore ? <RefreshCw className="spin" size={14} /> : null}
                {loadingMore ? "Loading…" : "Load 25 older requests"}
              </button>
            ) : null}
          </div>
        </>
      )}
    </section>
  );
}

function Metric({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string;
}) {
  return (
    <article>
      <span>{icon}</span>
      <div>
        <small>{label}</small>
        <strong>{value}</strong>
      </div>
    </article>
  );
}

function UsageChart({
  title,
  data,
  empty,
}: {
  title: string;
  data: Breakdown[];
  empty?: string;
}) {
  const chartData = data.map((item) => ({
    ...item,
    chartLabel: compactChartLabel(item.label),
  }));
  return (
    <article>
      <h4>{title}</h4>
      {data.length ? (
        <div className="usage-chart" role="img" aria-label={title}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={chartData}
              layout="vertical"
              margin={{ top: 4, right: 8, bottom: 4, left: 4 }}
              accessibilityLayer
            >
              <CartesianGrid stroke="var(--border)" horizontal={false} />
              <XAxis
                type="number"
                tick={{ fill: "var(--fg-muted)", fontSize: 10 }}
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                type="category"
                dataKey="chartLabel"
                width={120}
                tick={{ fill: "var(--fg-muted)", fontSize: 10 }}
                axisLine={false}
                tickLine={false}
              />
              <Tooltip
                cursor={{ fill: "var(--brand-tint)" }}
                contentStyle={{
                  background: "var(--surface-raised)",
                  border: 0,
                  borderRadius: 12,
                  boxShadow: "var(--mat-shadow-raised)",
                  fontSize: 12,
                }}
              />
              <Bar
                dataKey="requests"
                fill="var(--brand)"
                radius={[0, 8, 8, 0]}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <p>{empty ?? "No data yet."}</p>
      )}
    </article>
  );
}

function compactChartLabel(label: string) {
  if (label.startsWith("/ghanageo.v1.")) {
    return `gRPC · ${label.split("/").at(-1)}`;
  }
  return label
    .replace(" /v1/", " /")
    .replace("{id}", ":id")
    .replace("region:", "region · ")
    .replace("district:", "district · ");
}

function UsageSkeleton() {
  return (
    <div className="usage-skeleton" aria-label="Loading usage analytics">
      <div>
        {[0, 1, 2, 3].map((item) => (
          <i key={item} />
        ))}
      </div>
      <span />
      <span />
      <span />
    </div>
  );
}

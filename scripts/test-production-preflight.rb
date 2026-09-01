#!/usr/bin/env ruby
# frozen_string_literal: true

require "open3"
require "tempfile"

ROOT = File.expand_path("..", __dir__)
PREFLIGHT = File.join(__dir__, "production-preflight.rb")
SECRET_SENTINEL = "sensitive-preflight-sentinel"

def run_preflight(*arguments)
  Open3.capture3("ruby", PREFLIGHT, *arguments, chdir: ROOT)
end

def assert_result(label, success:, arguments:, output_includes: nil)
  stdout, stderr, status = run_preflight(*arguments)
  output = stdout + stderr
  abort "#{label}: expected success=#{success}, got exit #{status.exitstatus}\n#{output}" unless status.success? == success
  abort "#{label}: expected output to include #{output_includes.inspect}" if output_includes && !output.include?(output_includes)
  abort "#{label}: leaked a secret value" if output.include?(SECRET_SENTINEL)
end

def with_env(contents)
  Tempfile.create(["ghanageo-production", ".env"]) do |file|
    file.write(contents)
    file.flush
    yield file.path
  end
end

def without_line(contents, key)
  contents.lines.reject { |line| line.start_with?("#{key}=") }.join
end

assert_result(
  "default invocation fails closed",
  success: false,
  arguments: [],
  output_includes: "at least one --env PATH is required"
)
assert_result(
  "blueprint-only schema lint",
  success: true,
  arguments: ["--blueprint-only"],
  output_includes: "blueprint-only preflight passed"
)

api_env = <<~ENV
  GHANAGEO_ENV=production
  GHANAGEO_SERVE_MODE=http
  API_TRUST_PROXY_HEADERS=true
  API_REQUIRE_HTTPS=true
  API_PASSKEY_RPID=digitalghana.dev
  API_PASSKEY_ORIGINS=https://console-geo.digitalghana.dev,https://admin-geo.digitalghana.dev
  MONGO_URI=#{SECRET_SENTINEL}-mongo
  REDIS_URL=#{SECRET_SENTINEL}-redis
  TYPESENSE_URL=https://typesense.invalid
  TYPESENSE_API_KEY=#{SECRET_SENTINEL}-typesense
  INTERNAL_SERVICE_TOKEN=#{SECRET_SENTINEL}-internal
  RESEND_API_KEY=#{SECRET_SENTINEL}-resend
  RESEND_FROM_EMAIL=noreply@digitalghana.dev
  GHANAGEO_PORTAL_URL=https://console-geo.digitalghana.dev
  API_TRUSTED_PROXY_CIDRS=192.0.2.0/24
  OTEL_EXPORTER_OTLP_ENDPOINT=https://otel.invalid
  OTEL_EXPORTER_OTLP_HEADERS=#{SECRET_SENTINEL}-otel
  SECURITY_ALERT_WEBHOOK_URL=https://alerts.invalid/hook
  SECURITY_ALERT_WEBHOOK_SECRET=#{SECRET_SENTINEL}-alert
  SENTRY_DSN=https://public@sentry.invalid/1
ENV

worker_env = <<~ENV
  GHANAGEO_ENV=production
  MONGO_URI=#{SECRET_SENTINEL}-mongo
  REDIS_URL=#{SECRET_SENTINEL}-redis
  TYPESENSE_URL=https://typesense.invalid
  TYPESENSE_API_KEY=#{SECRET_SENTINEL}-typesense
  OTEL_EXPORTER_OTLP_ENDPOINT=https://otel.invalid
  OTEL_EXPORTER_OTLP_HEADERS=#{SECRET_SENTINEL}-otel
  SENTRY_DSN=https://public@sentry.invalid/2
ENV

frontend_env = <<~ENV
  NEXT_PUBLIC_GHANAGEO_API_URL=https://api-geo.digitalghana.dev/v1
  NEXT_PUBLIC_GHANAGEO_GRAPHQL_URL=https://api-geo.digitalghana.dev/graphql
  NEXT_PUBLIC_SITE_URL=https://geo.digitalghana.dev
  NEXT_PUBLIC_GHANAGEO_WEB_URL=https://geo.digitalghana.dev
  NEXT_PUBLIC_GHANAGEO_SANDBOX_URL=https://sandbox-geo.digitalghana.dev
  NEXT_PUBLIC_GHANAGEO_PORTAL_URL=https://console-geo.digitalghana.dev
  NEXT_PUBLIC_SENTRY_DSN=https://public@sentry.invalid/3
ENV

with_env(api_env) do |path|
  assert_result("populated API dotenv", success: true, arguments: ["--env", "ghanageo-api=#{path}"], output_includes: "production deployment preflight passed")
end
with_env(worker_env) do |path|
  assert_result("populated worker dotenv", success: true, arguments: ["--env", "ghanageo-worker=#{path}"], output_includes: "production deployment preflight passed")
end
with_env(frontend_env) do |path|
  assert_result("populated frontend dotenv", success: true, arguments: ["--env", "web=#{path}"], output_includes: "production deployment preflight passed")
end

with_env(api_env) do |api_path|
  with_env(worker_env) do |worker_path|
    stdout, stderr, status = Open3.capture3(
      "make", "production-preflight", "API_ENV=#{api_path}", "WORKER_ENV=#{worker_path}",
      chdir: ROOT
    )
    output = stdout + stderr
    abort "populated Make deployment gate failed\n#{output}" unless status.success?
    abort "populated Make deployment gate leaked a secret value" if output.include?(SECRET_SENTINEL)
  end
end

{
  "API telemetry omission" => [api_env, "OTEL_EXPORTER_OTLP_ENDPOINT"],
  "API telemetry authentication omission" => [api_env, "OTEL_EXPORTER_OTLP_HEADERS"],
  "API alert receiver omission" => [api_env, "SECURITY_ALERT_WEBHOOK_URL"],
  "API alert signing omission" => [api_env, "SECURITY_ALERT_WEBHOOK_SECRET"],
  "API Sentry omission" => [api_env, "SENTRY_DSN"],
  "worker telemetry omission" => [worker_env, "OTEL_EXPORTER_OTLP_ENDPOINT"],
  "worker telemetry authentication omission" => [worker_env, "OTEL_EXPORTER_OTLP_HEADERS"],
  "worker Sentry omission" => [worker_env, "SENTRY_DSN"],
  "frontend Sentry omission" => [frontend_env, "NEXT_PUBLIC_SENTRY_DSN"],
  "passkey RP ID omission" => [api_env, "API_PASSKEY_RPID"],
  "passkey origins omission" => [api_env, "API_PASSKEY_ORIGINS"]
}.each do |label, (contents, key)|
  service = label.start_with?("worker") ? "ghanageo-worker" : label.start_with?("frontend") ? "web" : "ghanageo-api"
  with_env(without_line(contents, key)) do |path|
    assert_result(label, success: false, arguments: ["--env", "#{service}=#{path}"], output_includes: "#{key} is missing")
  end
end

with_env(api_env.sub("192.0.2.0/24", "PASTE_RENDER_EDGE_PROXY_CIDRS")) do |path|
  assert_result(
    "placeholder production dotenv",
    success: false,
    arguments: ["--env", "ghanageo-api=#{path}"],
    output_includes: "missing, blank, or still a placeholder"
  )
end

puts "production preflight tests passed"

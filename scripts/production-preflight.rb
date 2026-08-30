#!/usr/bin/env ruby
# frozen_string_literal: true

require "optparse"
require "yaml"

options = { blueprint: "render.yaml", blueprint_only: false, env_files: [] }
OptionParser.new do |parser|
  parser.banner = "Usage: scripts/production-preflight.rb (--env SERVICE=PATH | --blueprint-only) [--blueprint PATH]"
  parser.on("--blueprint PATH", "Render Blueprint to validate") { |path| options[:blueprint] = path }
  parser.on("--blueprint-only", "Validate only the Blueprint (CI/schema lint, not a deploy gate)") { options[:blueprint_only] = true }
  parser.on("--env SERVICE=PATH", "Named production dotenv file to validate (repeatable)") { |spec| options[:env_files] << spec }
end.parse!

if options[:blueprint_only] && options[:env_files].any?
  warn "production preflight failed: --blueprint-only cannot be combined with --env"
  exit 2
end
if !options[:blueprint_only] && options[:env_files].empty?
  warn "production preflight failed: at least one --env PATH is required (use --blueprint-only only for CI/schema lint)"
  exit 2
end

errors = []

begin
  blueprint = YAML.safe_load(File.read(options[:blueprint]), aliases: false)
rescue Errno::ENOENT, Psych::Exception => error
  warn "production preflight failed: #{error.message}"
  exit 1
end

services = blueprint.is_a?(Hash) ? blueprint["services"] : nil
unless services.is_a?(Array)
  warn "production preflight failed: render.yaml must contain a services array"
  exit 1
end

service_by_name = services.each_with_object({}) do |service, result|
  unless service.is_a?(Hash) && service["name"].is_a?(String)
    errors << "every Render service must have a name"
    next
  end
  errors << "duplicate Render service name: #{service['name']}" if result.key?(service["name"])
  result[service["name"]] = service
end

required_values = {
  "ghanageo-api" => {
    "GHANAGEO_ENV" => "production",
    "GHANAGEO_SERVE_MODE" => "http",
    "API_TRUST_PROXY_HEADERS" => "true",
    "API_REQUIRE_HTTPS" => "true",
    "API_PASSKEY_RPID" => "digitalghana.dev",
    "API_PASSKEY_ORIGINS" => "https://console.geo.digitalghana.dev,https://admin.geo.digitalghana.dev",
    "GHANAGEO_PORTAL_URL" => "https://console.geo.digitalghana.dev"
  },
  "ghanageo-worker" => { "GHANAGEO_ENV" => "production" }
}.freeze

required_secrets = {
  "ghanageo-api" => %w[
    MONGO_URI REDIS_URL TYPESENSE_URL TYPESENSE_API_KEY INTERNAL_SERVICE_TOKEN
    RESEND_API_KEY RESEND_FROM_EMAIL API_TRUSTED_PROXY_CIDRS
    OTEL_EXPORTER_OTLP_ENDPOINT OTEL_EXPORTER_OTLP_HEADERS
    SECURITY_ALERT_WEBHOOK_URL SECURITY_ALERT_WEBHOOK_SECRET SENTRY_DSN
  ],
  "ghanageo-worker" => %w[
    MONGO_URI REDIS_URL TYPESENSE_URL TYPESENSE_API_KEY
    OTEL_EXPORTER_OTLP_ENDPOINT OTEL_EXPORTER_OTLP_HEADERS SENTRY_DSN
  ]
}.freeze

required_values.each do |service_name, expected_values|
  service = service_by_name[service_name]
  unless service
    errors << "missing Render service: #{service_name}"
    next
  end

  env_entries = service["envVars"]
  unless env_entries.is_a?(Array)
    errors << "#{service_name} must contain an envVars array"
    next
  end

  env_by_key = {}
  env_entries.each do |entry|
    unless entry.is_a?(Hash) && entry["key"].is_a?(String)
      errors << "#{service_name} contains an invalid envVars entry"
      next
    end
    errors << "#{service_name} declares #{entry['key']} more than once" if env_by_key.key?(entry["key"])
    env_by_key[entry["key"]] = entry
  end

  expected_values.each do |key, expected|
    actual = env_by_key.dig(key, "value")
    errors << "#{service_name} must set #{key}=#{expected}" unless actual.to_s == expected
  end

  required_secrets.fetch(service_name).each do |key|
    entry = env_by_key[key]
    if entry.nil?
      errors << "#{service_name} is missing secret declaration #{key}"
    elsif key == "REDIS_URL"
      expected_reference = {
        "name" => "ghanageo-redis",
        "type" => "keyvalue",
        "property" => "connectionString"
      }
      errors << "#{service_name} REDIS_URL must reference the managed ghanageo-redis connectionString" unless entry["fromService"] == expected_reference && !entry.key?("value") && !entry.key?("sync")
    elsif entry["sync"] != false || entry.key?("value")
      errors << "#{service_name} secret #{key} must use sync: false and must not contain a value"
    end
  end
end

redis = service_by_name["ghanageo-redis"]
if redis.nil?
  errors << "missing Render Key Value service: ghanageo-redis"
else
  errors << "ghanageo-redis must use type=keyvalue" unless redis["type"] == "keyvalue"
  errors << "ghanageo-redis must run in frankfurt with the API and worker" unless redis["region"] == "frankfurt"
  errors << "ghanageo-redis must deny public network access" unless redis["ipAllowList"] == []
  errors << "ghanageo-redis must use noeviction for queues and security counters" unless redis["maxmemoryPolicy"] == "noeviction"
  errors << "ghanageo-redis must enable journal-snapshot persistence" unless redis["persistenceMode"] == "journal-snapshot"
end

placeholder = /(?:paste_|replace[_-]?me|change[_-]?me|your[_-]|example|todo|localhost)/i
frontend_required = %w[
  NEXT_PUBLIC_GHANAGEO_API_URL NEXT_PUBLIC_GHANAGEO_GRAPHQL_URL NEXT_PUBLIC_SITE_URL
  NEXT_PUBLIC_GHANAGEO_WEB_URL NEXT_PUBLIC_GHANAGEO_SANDBOX_URL
  NEXT_PUBLIC_GHANAGEO_PORTAL_URL NEXT_PUBLIC_SENTRY_DSN
].freeze
frontend_services = %w[web sandbox portal admin].freeze

options[:env_files].each do |spec|
  service_name, path = spec.split("=", 2)
  if path.nil? || path.empty?
    errors << "--env must use SERVICE=PATH (received a path without a service name)"
    next
  end
  unless required_secrets.key?(service_name) || frontend_services.include?(service_name)
    errors << "unknown dotenv service #{service_name.inspect}; expected #{(required_secrets.keys + frontend_services).join(', ')}"
    next
  end

  values = {}
  begin
    File.foreach(path) do |line|
      line = line.chomp
      next if line.lstrip.start_with?("#") || line.strip.empty?

      match = line.match(/\A(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)\z/)
      next unless match

      value = match[2].strip
      value = value[1...-1] if value.length >= 2 && %w[' "].include?(value[0]) && value[-1] == value[0]
      values[match[1]] = value
    end
  rescue Errno::ENOENT => error
    errors << error.message
    next
  end

  required_dotenv = if required_secrets.key?(service_name)
                      # REDIS_URL is supplied by the validated Render Key Value
                      # fromService reference, so it must not be copied into a
                      # dotenv file or handled as a user-managed credential.
                      required_secrets.fetch(service_name).reject { |key| key == "REDIS_URL" } + required_values.fetch(service_name).keys
                    else
                      frontend_required
                    end
  required_dotenv.uniq.each do |key|
    value = values[key]
    errors << "#{path}: #{key} is missing, blank, or still a placeholder" if value.nil? || value.empty? || value.match?(placeholder)
  end
  if required_values.key?(service_name)
    required_values.fetch(service_name).each do |key, expected|
      errors << "#{path}: #{key} must equal its Render production value" unless values[key] == expected
    end
  end
end

if errors.any?
  warn "production preflight failed (#{errors.length} issue#{errors.length == 1 ? '' : 's'}):"
  errors.each { |error| warn "- #{error}" }
  exit 1
end

if options[:blueprint_only]
  puts "blueprint-only preflight passed: structure and required configuration names are valid"
else
  puts "production deployment preflight passed: blueprint and #{options[:env_files].length} dotenv file#{options[:env_files].length == 1 ? '' : 's'} are valid"
  puts "secret values were not printed"
end

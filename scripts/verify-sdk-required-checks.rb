#!/usr/bin/env ruby
# frozen_string_literal: true

require "json"
require "open3"

repository, revision = ARGV
abort "usage: verify-sdk-required-checks.rb <owner/repo> <sha>" unless repository && revision
required = {
  "Complete GEO-24.1M conformance matrix" => ".github/workflows/quality.yml",
  "SDK documentation drift and examples" => ".github/workflows/quality.yml"
}

60.times do |attempt|
  body, status = Open3.capture2e("gh", "api", "repos/#{repository}/commits/#{revision}/check-runs?per_page=100")
  abort body unless status.success?
  runs = JSON.parse(body).fetch("check_runs")
  state = required.map do |name, workflow_path|
    run = runs.select { |candidate| candidate.fetch("name") == name && candidate.dig("app", "slug") == "github-actions" }
              .max_by { |candidate| candidate.fetch("started_at", "") }
    next :wait unless run
    details = run.fetch("details_url", "")
    match = details.match(%r{/actions/runs/(\d+)/job/})
    abort "#{name}: untrusted check details URL" unless match
    workflow_body, workflow_status = Open3.capture2e("gh", "api", "repos/#{repository}/actions/runs/#{match[1]}")
    abort workflow_body unless workflow_status.success?
    workflow = JSON.parse(workflow_body)
    actual_path = workflow.fetch("path").sub(/@.*\z/, "")
    abort "#{name}: expected #{workflow_path}, got #{actual_path}" unless actual_path == workflow_path
    abort "#{name}: check is for #{workflow.fetch('head_sha')}, not #{revision}" unless workflow.fetch("head_sha") == revision
    run.fetch("status") == "completed" ? (run.fetch("conclusion") == "success" ? :pass : :fail) : :wait
  end
  exit 0 if state.all?(:pass)
  abort "a required SDK check failed" if state.include?(:fail)
  warn "waiting for trusted required checks (#{attempt + 1}/60)"
  sleep 10
end
abort "timed out waiting for trusted SDK checks"

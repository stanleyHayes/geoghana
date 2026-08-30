#!/usr/bin/env ruby
# frozen_string_literal: true

require "json"
require_relative "lib"

abort "usage: validate_matrix.rb REPORT_DIRECTORY" unless ARGV.length == 1
directory = File.expand_path(ARGV.fetch(0))
manifest = JSON.parse(File.read(File.join(GhanaGeo::Conformance::ROOT, "contracts/conformance/matrix.json")))
paths = Dir[File.join(directory, "**", "*.json")].sort
abort "matrix contains no reports" if paths.empty?

begin
  cases = GhanaGeo::Conformance.validate!
  reports = paths.map do |path|
    report = JSON.parse(File.read(path))
    next unless report["schemaVersion"] == 2 && report["sdk"].is_a?(Hash)
    GhanaGeo::Conformance.validate_report!(path, cases)
  end.compact
  abort "matrix contains no conformance reports" if reports.empty?

  manifest.fetch("runners").each do |runner|
    matches = reports.select do |report|
      report.dig("sdk", "name") == runner.fetch("name") &&
        report.dig("sdk", "language") == runner.fetch("language") &&
        report.dig("sdk", "supportedProtocols").sort == runner.fetch("protocols").sort
    end
    expected = runner.fetch("requiredReports")
    raise GhanaGeo::Conformance::ValidationError, "#{runner.fetch("language")}/#{runner.fetch("name")} produced #{matches.length} reports, expected #{expected}" unless matches.length == expected
  end

  expected_count = manifest.fetch("runners").sum { |runner| runner.fetch("requiredReports") }
  raise GhanaGeo::Conformance::ValidationError, "matrix contains unexpected reports" unless reports.length == expected_count
  reports.each do |report|
    raise GhanaGeo::Conformance::ValidationError, "matrix report contains fail or skip" unless report.dig("summary", "failed").zero? && report.dig("summary", "skipped").zero? && report.fetch("results").all? { |result| result["status"] == "passed" }
  end
  %w[apiVersion datasetVersion].each do |field|
    values = reports.map { |report| report.fetch(field) }.uniq
    raise GhanaGeo::Conformance::ValidationError, "matrix #{field} values differ: #{values.join(", ")}" unless values.length == 1
  end
  digests = reports.map { |report| report.dig("contract", "digest") }.uniq
  raise GhanaGeo::Conformance::ValidationError, "matrix contract digests differ" unless digests.length == 1

  observed_pairs = reports.flat_map { |report| report.fetch("evidence").map { |item| [item.fetch("caseId"), item.fetch("protocol")] } }.uniq
  expected_pairs = cases.select { |test_case| test_case["required"] }.flat_map { |test_case| test_case.fetch("protocols").map { |protocol| [test_case.fetch("id"), protocol] } }.uniq
  missing = expected_pairs - observed_pairs
  raise GhanaGeo::Conformance::ValidationError, "matrix misses operation-protocol evidence: #{missing.map { |pair| pair.join("/") }.join(", ")}" unless missing.empty?

  parity = cases.find { |test_case| Array(test_case.dig("expect", "semantics")).include?("protocolResultsEquivalent") }
  raise GhanaGeo::Conformance::ValidationError, "matrix lacks cross-protocol parity case" unless parity
  parity_protocols = observed_pairs.select { |case_id, _| case_id == parity.fetch("id") }.map(&:last).uniq.sort
  raise GhanaGeo::Conformance::ValidationError, "matrix parity evidence is incomplete" unless parity_protocols == parity.fetch("protocols").sort

  puts "complete SDK matrix valid: #{reports.length} reports, #{observed_pairs.length} distinct case/protocol evidence pairs"
rescue GhanaGeo::Conformance::ValidationError, JSON::ParserError, KeyError => error
  warn "SDK matrix invalid: #{error.message}"
  exit 1
end

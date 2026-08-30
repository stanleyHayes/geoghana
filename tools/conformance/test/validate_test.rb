# frozen_string_literal: true

require "minitest/autorun"
require "tempfile"
require_relative "../lib"

class ConformanceValidationTest < Minitest::Test
  def setup
    @cases = GhanaGeo::Conformance.validate!
  end

  def test_repository_contract_is_complete_and_multi_protocol
    assert_equal 16, @cases.count { |test_case| test_case.dig("expect", "outcome") == "error" }
    assert_equal GhanaGeo::Conformance.operation_inventory.map { |entry| entry["operation"] }.sort,
                 @cases.select { |test_case| test_case["required"] }.map { |test_case| test_case["operation"] }.uniq.sort
    generated_from = GhanaGeo::Conformance.load_yaml(File.join(GhanaGeo::Conformance::CONTRACT_DIR, "operations.generated.yaml"))["generatedFrom"]
    assert_includes generated_from, "contracts/graphql/schema.graphql"
    assert @cases.any? { |test_case| test_case.dig("expect", "semantics")&.include?("protocolResultsEquivalent") }
    assert @cases.any? { |test_case| test_case.dig("expect", "semantics")&.include?("preservesGhanaianOrthography") }
  end

  def test_declared_case_schema_rejects_unknown_and_missing_fields
    schema = JSON.parse(File.read(File.join(GhanaGeo::Conformance::CONTRACT_DIR, "schema.json")))
    validator = GhanaGeo::Conformance::SchemaValidator.new(schema)
    invalid = { "version" => 1, "cases" => [deep_copy(@cases.first).merge("surprise" => true)] }
    assert_raises(GhanaGeo::Conformance::ValidationError) { validator.validate!(invalid) }
    invalid["cases"][0].delete("surprise")
    invalid["cases"][0]["expect"].delete("quotaCost")
    assert_raises(GhanaGeo::Conformance::ValidationError) { validator.validate!(invalid) }
  end

  def test_error_catalog_coverage_must_be_required
    cases = deep_copy(@cases)
    cases.find { |test_case| test_case.dig("expect", "error", "code") == "INTERNAL" }["required"] = false
    error = assert_raises(GhanaGeo::Conformance::ValidationError) { GhanaGeo::Conformance.validate_error_coverage!(cases) }
    assert_match(/required Appendix B error coverage missing/, error.message)
  end

  def test_report_schema_rejects_unknown_properties
    report = valid_report.merge("unexpected" => true)
    assert_report_rejected(report, /unknown properties/)
  end

  def test_report_rejects_failed_skipped_and_missing_required_cases
    %w[failed skipped].each do |status|
      report = valid_report
      report["results"][0]["status"] = status
      recompute_summary(report)
      assert_report_rejected(report, /required cases did not pass/)
    end
    report = valid_report
    report["results"].shift
    recompute_summary(report)
    assert_report_rejected(report, /omits required cases/)
  end

  def test_report_rejects_unknown_and_duplicate_case_ids
    report = valid_report
    report["results"] << { "caseId" => "unknown.case", "status" => "passed", "protocols" => ["rest"] }
    recompute_summary(report)
    assert_report_rejected(report, /unknown case ids/)

    report = valid_report
    report["results"] << deep_copy(report["results"].first)
    recompute_summary(report)
    assert_report_rejected(report, /duplicates case ids/)
  end

  def test_report_messages_reject_secrets_and_normalization_redacts_them
    report = valid_report
    report["results"][0]["message"] = "Authorization: Bearer gh_live_1234567890abcdef"
    assert_report_rejected(report, /key-shaped secret/)
    normalized = GhanaGeo::Conformance.normalize_report(report)
    assert_includes normalized, GhanaGeo::Conformance::REDACTED
    refute_match GhanaGeo::Conformance::SECRET_PATTERN, normalized
    %w[
      ghp_123456789012345678901234567890123456
      github_pat_123456789012345678901234567890
      npm_123456789012345678901234567890
      glpat-123456789012345678901234567890
      sk-123456789012345678901234567890
      AKIA1234567890ABCDEF
    ].each { |secret| assert_match GhanaGeo::Conformance::SECRET_PATTERN, secret }
  end

  def test_documented_secret_placeholders_are_allowed
    ["Bearer YOUR_API_KEY", "api_key=YOUR_API_KEY", "password=REDACTED"].each do |placeholder|
      refute_match GhanaGeo::Conformance::SECRET_PATTERN, placeholder
    end
  end

  def test_binary_secret_scan_extracts_printable_tokens_without_random_false_positives
    assert GhanaGeo::Conformance.contains_secret?("\x00compiled\x00AKIA1234567890ABCDEF\x00".b)
    refute GhanaGeo::Conformance.contains_secret?("\x00\x01\x02ordinary-binary-symbol\x00Bearer YOUR_API_KEY\x00".b)
  end

  def test_report_is_bound_to_contract_runner_and_pair_evidence
    report = valid_report
    report["contract"]["digest"] = "0" * 64
    assert_report_rejected(report, /contract digest/)

    report = valid_report
    report["evidence"].shift
    report["evidenceDigest"] = Digest::SHA256.hexdigest(GhanaGeo::Conformance.canonical_json(report["evidence"]))
    assert_report_rejected(report, /evidence does not bind/)

    report = valid_report
    report["evidence"][0]["requestDigest"] = "Bearer gh_test_1234567890"
    assert_report_rejected(report, /does not match|key-shaped secret/)
  end

  def test_report_normalization_is_byte_stable
    report = valid_report
    report["results"].reverse!
    first = GhanaGeo::Conformance.normalize_report(report)
    second = GhanaGeo::Conformance.normalize_report(JSON.parse(first))
    assert_equal first, second
  end

  private

  def deep_copy(value)
    Marshal.load(Marshal.dump(value))
  end

  def valid_report
    supported = %w[rest grpc graphql]
    results = @cases.map { |test_case| { "caseId" => test_case["id"], "status" => "passed", "protocols" => test_case["protocols"] & supported } }
    evidence = results.flat_map do |result|
      result["protocols"].map { |protocol| { "caseId" => result["caseId"], "protocol" => protocol, "requestDigest" => "a" * 64, "responseDigest" => "b" * 64 } }
    end
    {
      "schemaVersion" => 2,
      "contract" => { "digest" => GhanaGeo::Conformance.contract_digest, "runnerVersion" => "1.0.0" },
      "sdk" => { "language" => "test", "name" => "test", "version" => "0.0.0", "supportedProtocols" => supported },
      "apiVersion" => "v1",
      "datasetVersion" => "2026.08.3-ulid",
      "summary" => { "passed" => results.length, "failed" => 0, "skipped" => 0 },
      "evidenceDigest" => Digest::SHA256.hexdigest(GhanaGeo::Conformance.canonical_json(evidence)),
      "evidence" => evidence,
      "results" => results
    }
  end

  def recompute_summary(report)
    counts = report["results"].group_by { |result| result["status"] }.transform_values(&:length)
    report["summary"] = %w[passed failed skipped].each_with_object({}) { |status, summary| summary[status] = counts.fetch(status, 0) }
  end

  def assert_report_rejected(report, pattern)
    Tempfile.create(["report", ".json"]) do |file|
      file.write(JSON.generate(report))
      file.flush
      error = assert_raises(GhanaGeo::Conformance::ValidationError) { GhanaGeo::Conformance.validate_report!(file.path, @cases) }
      assert_match pattern, error.message
    end
  end
end

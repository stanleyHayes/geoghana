# frozen_string_literal: true

require "json"
require "minitest/autorun"
require "open3"
require "tmpdir"
require_relative "../lib"

class MatrixTest < Minitest::Test
  def test_complete_matrix_accepts_every_declared_runner_and_protocol_pair
    cases = GhanaGeo::Conformance.validate!
    manifest = JSON.parse(File.read(File.join(GhanaGeo::Conformance::ROOT, "contracts/conformance/matrix.json")))
    Dir.mktmpdir("ghanageo-matrix-test") do |directory|
      index = 0
      manifest.fetch("runners").each do |runner|
        runner.fetch("requiredReports").times do
          protocols = runner.fetch("protocols")
          results = cases.map do |test_case|
            covered = test_case.fetch("protocols") & protocols
            { "caseId" => test_case.fetch("id"), "status" => "passed", "protocols" => covered } unless covered.empty?
          end.compact
          evidence = results.flat_map { |result| result.fetch("protocols").map { |protocol| { "caseId" => result.fetch("caseId"), "protocol" => protocol, "requestDigest" => "a" * 64, "responseDigest" => "b" * 64 } } }
          report = { "schemaVersion" => 2, "contract" => { "digest" => GhanaGeo::Conformance.contract_digest, "runnerVersion" => "1.0.0" }, "sdk" => { "language" => runner.fetch("language"), "name" => runner.fetch("name"), "version" => "0.0.0", "supportedProtocols" => protocols }, "apiVersion" => "v1", "datasetVersion" => "2026.08.3-ulid", "summary" => { "passed" => results.length, "failed" => 0, "skipped" => 0 }, "evidenceDigest" => Digest::SHA256.hexdigest(GhanaGeo::Conformance.canonical_json(evidence)), "evidence" => evidence, "results" => results }
          File.write(File.join(directory, "report-#{index += 1}.json"), JSON.generate(report))
        end
      end
      output, status = Open3.capture2e(RbConfig.ruby, File.join(__dir__, "..", "validate_matrix.rb"), directory)
      assert status.success?, output
      assert_includes output, "complete SDK matrix valid"
    end
  end
end

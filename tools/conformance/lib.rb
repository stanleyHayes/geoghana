# frozen_string_literal: true

require "json"
require "digest"
require "open3"
require "rbconfig"
require "yaml"

module GhanaGeo
  module Conformance
    ROOT = File.expand_path("../..", __dir__)
    CONTRACT_DIR = File.join(ROOT, "contracts/conformance")
    CASE_DIR = File.join(CONTRACT_DIR, "cases")
    REQUIRED_ERROR_FIELDS = %w[code message requestId docs].freeze
    SECRET_PATTERN = /(?:
      gh_(?:live|test)_[A-Za-z0-9_-]{8,}|
      gh[pousr]_[A-Za-z0-9]{20,}|
      github_pat_[A-Za-z0-9_]{20,}|
      npm_[A-Za-z0-9]{20,}|
      glpat-[A-Za-z0-9_-]{20,}|
      xox[baprs]-[A-Za-z0-9-]{10,}|
      AKIA[A-Z0-9]{16}|
      sk-[A-Za-z0-9_-]{20,}|
      (?:authorization\s*:\s*)?bearer\s+(?!(?:YOUR_API_KEY|REDACTED|TOKEN_HERE)\b|\$\{)[A-Za-z0-9._~-]{12,}|
      (?:api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret|password)\s*[=:]\s*(?:
        ["'](?!(?:YOUR_API_KEY|REDACTED|TOKEN_HERE)\b)[^"']{12,}["']|
        (?!(?:YOUR_API_KEY|REDACTED|TOKEN_HERE|process\.env|options\.)\b)[A-Za-z0-9_-]{16,}
      )
    )/ix.freeze
    REDACTED = "[REDACTED]".freeze

    def self.contains_secret?(content)
      searchable = if content.include?("\0")
        content.scan(/[\x20-\x7e]{8,}/).join("\n")
      else
        content
      end
      searchable.match?(SECRET_PATTERN)
    end

    class ValidationError < StandardError; end

    class SchemaValidator
      TYPES = {
        "object" => Hash, "array" => Array, "string" => String,
        "integer" => Integer, "number" => Numeric,
        "boolean" => [TrueClass, FalseClass]
      }.freeze

      def initialize(schema)
        @root = schema
      end

      def validate!(value, schema = @root, path = "$")
        if schema["$ref"]
          return validate!(value, resolve(schema["$ref"]), path)
        end
        if schema["oneOf"]
          matches = schema["oneOf"].count do |candidate|
            validate!(value, candidate, path)
            true
          rescue ValidationError
            false
          end
          fail_at(path, "must match exactly one schema (matched #{matches})") unless matches == 1
          return true
        end

        fail_at(path, "must equal #{schema["const"].inspect}") if schema.key?("const") && value != schema["const"]
        fail_at(path, "must be one of #{schema["enum"].inspect}") if schema["enum"] && !schema["enum"].include?(value)
        validate_type!(value, schema["type"], path) if schema["type"]

        if value.is_a?(Hash)
          Array(schema["required"]).each { |key| fail_at(path, "is missing required property #{key}") unless value.key?(key) }
          properties = schema.fetch("properties", {})
          if schema["additionalProperties"] == false
            unknown = value.keys - properties.keys
            fail_at(path, "has unknown properties: #{unknown.join(", ")}") unless unknown.empty?
          end
          value.each { |key, child| validate!(child, properties[key], "#{path}.#{key}") if properties[key] }
        elsif value.is_a?(Array)
          fail_at(path, "must contain at least #{schema["minItems"]} items") if schema["minItems"] && value.length < schema["minItems"]
          fail_at(path, "must contain unique items") if schema["uniqueItems"] && value.uniq.length != value.length
          value.each_with_index { |child, index| validate!(child, schema["items"], "#{path}[#{index}]") } if schema["items"]
        elsif value.is_a?(String)
          fail_at(path, "is shorter than #{schema["minLength"]}") if schema["minLength"] && value.length < schema["minLength"]
          fail_at(path, "does not match #{schema["pattern"]}") if schema["pattern"] && !Regexp.new(schema["pattern"]).match?(value)
        elsif value.is_a?(Numeric)
          fail_at(path, "is less than #{schema["minimum"]}") if schema["minimum"] && value < schema["minimum"]
          fail_at(path, "is greater than #{schema["maximum"]}") if schema["maximum"] && value > schema["maximum"]
        end
        true
      end

      private

      def resolve(reference)
        raise ValidationError, "unsupported external schema reference #{reference}" unless reference.start_with?("#/")

        reference.delete_prefix("#/").split("/").reduce(@root) { |node, segment| node.fetch(segment) }
      end

      def validate_type!(value, type, path)
        expected = TYPES.fetch(type) { raise ValidationError, "unsupported schema type #{type}" }
        valid = Array(expected).any? { |klass| value.is_a?(klass) }
        valid = false if type == "integer" && value.is_a?(TrueClass)
        fail_at(path, "must be #{type}") unless valid
      end

      def fail_at(path, message)
        raise ValidationError, "#{path} #{message}"
      end
    end

    module_function

    def load_yaml(path)
      YAML.safe_load(File.read(path), [], [], false)
    rescue Psych::Exception => error
      raise ValidationError, "#{relative(path)} is not valid YAML: #{error.message}"
    end

    def relative(path)
      path.sub(ROOT + "/", "")
    end

    def operation_inventory
      load_yaml(File.join(CONTRACT_DIR, "operations.generated.yaml")).fetch("operations")
    end

    def error_catalog
      load_yaml(File.join(ROOT, "contracts/errors/catalog.yaml")).fetch("errors")
    end

    def contract_digest
      paths = Dir[File.join(CONTRACT_DIR, "**", "*")].select { |path| File.file?(path) }.sort + [File.join(ROOT, "contracts/errors/catalog.yaml")]
      Digest::SHA256.hexdigest(paths.map { |path| "#{relative(path)}\0#{File.binread(path)}" }.join("\0"))
    end

    def canonical_json(value)
      case value
      when Hash then "{" + value.keys.sort.map { |key| "#{JSON.generate(key)}:#{canonical_json(value[key])}" }.join(",") + "}"
      when Array then "[" + value.map { |child| canonical_json(child) }.join(",") + "]"
      else JSON.generate(value)
      end
    end

    def case_documents
      Dir[File.join(CASE_DIR, "*.yaml")].sort.map { |path| [path, load_yaml(path)] }
    end

    def load_cases
      case_documents.flat_map { |_path, document| document.fetch("cases") }
    end

    def validate!
      validate_generated_inventory!
      case_schema = JSON.parse(File.read(File.join(CONTRACT_DIR, "schema.json")))
      JSON.parse(File.read(File.join(CONTRACT_DIR, "report-schema.json")))
      case_documents.each do |path, document|
        assert(!File.read(path).match?(SECRET_PATTERN), "#{relative(path)} contains a key-shaped secret")
        SchemaValidator.new(case_schema).validate!(document)
      end
      cases = load_cases
      validate_case_semantics!(cases)
      validate_unique_ids!(cases)
      validate_operation_coverage!(cases)
      validate_protocol_claims!(cases)
      validate_error_coverage!(cases)
      cases
    rescue JSON::ParserError => error
      raise ValidationError, "invalid JSON schema: #{error.message}"
    rescue KeyError => error
      raise ValidationError, "conformance document is incomplete: #{error.message}"
    end

    def validate_generated_inventory!
      output, status = Open3.capture2e(RbConfig.ruby, File.join(__dir__, "generate.rb"), "--check")
      assert(status.success?, output.strip)
    end

    def validate_case_semantics!(cases)
      cases.each do |test_case|
        id = test_case["id"]
        cost = test_case.dig("expect", "quotaCost")
        exact = cost.key?("units")
        ranged = cost.key?("minimumUnits") && cost.key?("maximumUnits")
        dynamic = cost["class"] == "dynamic"
        assert(exact || ranged || dynamic, "#{id} must declare exact, ranged, or dynamic quota cost")
        assert(cost["minimumUnits"] <= cost["maximumUnits"], "#{id} quota range is inverted") if ranged
        next unless test_case.dig("expect", "outcome") == "error"

        required_fields = test_case.dig("expect", "error", "requiredFields")
        missing_fields = REQUIRED_ERROR_FIELDS - required_fields
        assert(missing_fields.empty?, "#{id} error omits required fields: #{missing_fields.join(", ")}")
      end
    end

    def validate_unique_ids!(cases)
      duplicates = cases.group_by { |test_case| test_case["id"] }.select { |_id, entries| entries.length > 1 }.keys
      assert(duplicates.empty?, "duplicate case ids: #{duplicates.join(", ")}")
    end

    def validate_operation_coverage!(cases)
      operations = operation_inventory.map { |entry| entry.fetch("operation") }
      required_operations = cases.select { |test_case| test_case["required"] }.map { |test_case| test_case["operation"] }.uniq
      assert((operations - required_operations).empty?, "required operation coverage missing: #{(operations - required_operations).join(", ")}")
      assert((cases.map { |test_case| test_case["operation"] }.uniq - operations).empty?, "cases reference operations absent from generated contracts")
    end

    def validate_protocol_claims!(cases)
      inventory = operation_inventory.each_with_object({}) { |entry, index| index[entry.fetch("operation")] = entry }
      cases.each do |test_case|
        supported = %w[rest grpc graphql].select { |protocol| inventory.fetch(test_case["operation"]).key?(protocol) }
        impossible = test_case["protocols"] - supported
        assert(impossible.empty?, "#{test_case["id"]} claims unsupported protocols: #{impossible.join(", ")}")
      end
      inventory.each do |operation, entry|
        supported = %w[rest grpc graphql].select { |protocol| entry.key?(protocol) }
        covered = cases.select { |test_case| test_case["required"] && test_case["operation"] == operation }.flat_map { |test_case| test_case["protocols"] }.uniq
        assert((supported - covered).empty?, "#{operation} lacks required protocol coverage: #{(supported - covered).join(", ")}")
      end
    end

    def validate_error_coverage!(cases)
      catalog = error_catalog.each_with_object({}) { |entry, index| index[entry.fetch("code")] = entry }
      error_cases = cases.select { |test_case| test_case.dig("expect", "outcome") == "error" }
      required_error_cases = error_cases.select { |test_case| test_case["required"] }
      covered = required_error_cases.map { |test_case| test_case.dig("expect", "error", "code") }
      assert((catalog.keys - covered).empty?, "required Appendix B error coverage missing: #{(catalog.keys - covered).join(", ")}")
      all_codes = error_cases.map { |test_case| test_case.dig("expect", "error", "code") }
      assert((all_codes - catalog.keys).empty?, "cases reference errors absent from catalog: #{(all_codes - catalog.keys).join(", ")}")
      error_cases.each do |test_case|
        expected = test_case.fetch("expect")
        error = expected.fetch("error")
        source = catalog.fetch(error.fetch("code"))
        assert(expected["httpStatus"] == source["http"], "#{test_case["id"]} HTTP status differs from error catalog")
        assert(error["grpcStatus"] == source["grpc"], "#{test_case["id"]} gRPC status differs from error catalog")
        Array(source["details"]).each do |detail|
          assert(error["requiredFields"].include?("details.#{detail}"), "#{test_case["id"]} omits catalog detail #{detail}")
        end
      end
    end

    def validate_report!(path, cases = validate!)
      report = JSON.parse(File.read(path))
      schema = JSON.parse(File.read(File.join(CONTRACT_DIR, "report-schema.json")))
      SchemaValidator.new(schema).validate!(report)
      assert(report.dig("contract", "digest") == contract_digest, "report contract digest does not match the checked contract")
      case_ids = cases.map { |test_case| test_case["id"] }
      results = report.fetch("results")
      result_ids = results.map { |result| result["caseId"] }
      duplicates = result_ids.group_by(&:itself).select { |_id, entries| entries.length > 1 }.keys
      assert(duplicates.empty?, "report duplicates case ids: #{duplicates.join(", ")}")
      assert((result_ids - case_ids).empty?, "report contains unknown case ids: #{(result_ids - case_ids).join(", ")}")
      supported = report.dig("sdk", "supportedProtocols")
      applicable = cases.select { |test_case| !(test_case["protocols"] & supported).empty? }
      required_ids = applicable.select { |test_case| test_case["required"] }.map { |test_case| test_case["id"] }
      assert((required_ids - result_ids).empty?, "report omits required cases: #{(required_ids - result_ids).join(", ")}")
      assert((result_ids - applicable.map { |test_case| test_case["id"] }).empty?, "report includes cases with no supported protocol")
      unacceptable = results.select { |result| required_ids.include?(result["caseId"]) && result["status"] != "passed" }
      assert(unacceptable.empty?, "required cases did not pass: #{unacceptable.map { |result| result["caseId"] }.join(", ")}")
      by_case = cases.each_with_object({}) { |test_case, index| index[test_case["id"]] = test_case }
      results.each do |result|
        expected_protocols = by_case.fetch(result["caseId"])["protocols"] & supported
        assert(result["protocols"].sort == expected_protocols.sort, "#{result["caseId"]} does not cover every supported operation-protocol pair")
      end
      evidence = report.fetch("evidence")
      assert(report["evidenceDigest"] == Digest::SHA256.hexdigest(canonical_json(evidence)), "report evidence digest is invalid")
      evidence_pairs = evidence.map { |entry| [entry["caseId"], entry["protocol"]] }
      assert(evidence_pairs.uniq.length == evidence_pairs.length, "report duplicates evidence pairs")
      expected_pairs = results.flat_map { |result| result["protocols"].map { |protocol| [result["caseId"], protocol] } }
      assert(evidence_pairs.sort == expected_pairs.sort, "report evidence does not bind every executed operation-protocol pair")
      assert(!JSON.generate(report).match?(SECRET_PATTERN), "report contains a key-shaped secret")
      counts = results.group_by { |result| result["status"] }.transform_values(&:length)
      %w[passed failed skipped].each { |status| assert(report.dig("summary", status) == counts.fetch(status, 0), "report summary #{status} is incorrect") }
      report
    rescue JSON::ParserError => error
      raise ValidationError, "report is not valid JSON: #{error.message}"
    end

    def redact_secrets(value)
      case value
      when Hash then value.each_with_object({}) { |(key, child), result| result[key] = redact_secrets(child) }
      when Array then value.map { |child| redact_secrets(child) }
      when String then value.gsub(SECRET_PATTERN, REDACTED)
      else value
      end
    end

    def normalize_report(report)
      normalized = redact_secrets(Marshal.load(Marshal.dump(report)))
      normalized["results"] = Array(normalized["results"]).sort_by { |result| result.fetch("caseId") }
      normalized["evidence"] = Array(normalized["evidence"]).sort_by { |entry| [entry.fetch("caseId"), entry.fetch("protocol")] }
      normalized["evidenceDigest"] = Digest::SHA256.hexdigest(canonical_json(normalized["evidence"]))
      counts = normalized["results"].group_by { |result| result["status"] }.transform_values(&:length)
      normalized["summary"] = %w[passed failed skipped].each_with_object({}) { |status, summary| summary[status] = counts.fetch(status, 0) }
      JSON.pretty_generate(normalized) + "\n"
    end

    def assert(condition, message)
      raise ValidationError, message unless condition
    end
  end
end

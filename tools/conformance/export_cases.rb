#!/usr/bin/env ruby
# frozen_string_literal: true

require_relative "lib"

cases = GhanaGeo::Conformance.validate!
puts JSON.generate({ "schemaVersion" => 1, "contractDigest" => GhanaGeo::Conformance.contract_digest, "cases" => cases.sort_by { |test_case| test_case.fetch("id") } })

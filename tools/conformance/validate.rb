#!/usr/bin/env ruby
# frozen_string_literal: true

require "optparse"
require_relative "lib"

options = {}
OptionParser.new do |parser|
  parser.on("--report PATH", "Also validate an SDK report") { |path| options[:report] = path }
end.parse!

begin
  cases = GhanaGeo::Conformance.validate!
  GhanaGeo::Conformance.validate_report!(options[:report], cases) if options[:report]
  required = cases.count { |test_case| test_case["required"] }
  errors = cases.count { |test_case| test_case.dig("expect", "outcome") == "error" }
  puts "conformance contract valid: #{required} required cases, #{errors} error codes"
rescue GhanaGeo::Conformance::ValidationError => error
  warn "conformance contract invalid: #{error.message}"
  exit 1
end

#!/usr/bin/env ruby
# frozen_string_literal: true

require_relative "lib"
require "tempfile"

path = ARGV.fetch(0) { abort "usage: ruby tools/conformance/normalize_report.rb REPORT.json" }

begin
  normalized = GhanaGeo::Conformance.normalize_report(JSON.parse(File.read(path)))
  Tempfile.create(["normalized-conformance-report", ".json"]) do |file|
    file.write(normalized)
    file.flush
    GhanaGeo::Conformance.validate_report!(file.path)
  end
  puts normalized
rescue GhanaGeo::Conformance::ValidationError, JSON::ParserError => error
  warn "cannot normalize report: #{error.message}"
  exit 1
end

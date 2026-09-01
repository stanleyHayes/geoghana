#!/usr/bin/env sh
set -eu

curl --fail --silent --show-error \
  "${GHANAGEO_API_URL:-https://api-geo.digitalghana.dev}/v1/search?q=Kumasi&limit=5"

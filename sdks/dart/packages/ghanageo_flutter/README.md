# GhanaGeo Flutter

Accessible Flutter companion widgets for the Dart-only `ghanageo` client.

`GhanaGeoLocationPicker` is a purpose-built text field and semantic listbox. It
debounces remote autocomplete, aborts superseded requests, rejects stale
responses, supports arrow-key selection, Enter and Escape, and announces
loading, errors and result counts to assistive technology.

Run `dart tool/verify.dart` with Flutter 3.47.2. The verifier creates a temporary
monorepo dependency override, tests the widget, installs isolated staged core
and companion payloads into a clean Flutter consumer, validates that the dry run
has zero warnings and only the expected pre-publication override hint, and then
removes the override. Release automation must publish `ghanageo` before
`ghanageo_flutter`.

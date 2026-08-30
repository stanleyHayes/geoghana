import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:ghanageo/ghanageo.dart';

/// An accessible, cancellable async search field for GhanaGeo places.
class GhanaGeoLocationPicker extends StatefulWidget {
  const GhanaGeoLocationPicker({
    required this.client,
    required this.onSelected,
    this.initialValue,
    this.label = 'Search for a location',
    this.hintText = 'Try Kumasi or Osu',
    this.debounce = const Duration(milliseconds: 250),
    this.limit = 8,
    super.key,
  });

  final GhanaGeoClient client;
  final ValueChanged<Place> onSelected;
  final Place? initialValue;
  final String label;
  final String hintText;
  final Duration debounce;
  final int limit;

  @override
  State<GhanaGeoLocationPicker> createState() => _GhanaGeoLocationPickerState();
}

class _GhanaGeoLocationPickerState extends State<GhanaGeoLocationPicker> {
  late final TextEditingController _controller;
  final FocusNode _focusNode = FocusNode();
  Timer? _timer;
  CancelToken? _request;
  List<SearchResult> _results = const [];
  bool _loading = false;
  String? _error;
  int _activeIndex = -1;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialValue?.name ?? '');
  }

  @override
  void didUpdateWidget(covariant GhanaGeoLocationPicker oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.initialValue?.id != widget.initialValue?.id ||
        oldWidget.initialValue?.name != widget.initialValue?.name) {
      final text = widget.initialValue?.name ?? '';
      _controller
        ..text = text
        ..selection = TextSelection.collapsed(offset: text.length);
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    _request?.cancel();
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _changed(String value) {
    _timer?.cancel();
    _request?.cancel();
    final generation = ++_generation;
    final query = value.trim();
    if (query.length < 2) {
      setState(() {
        _results = const [];
        _loading = false;
        _error = null;
        _activeIndex = -1;
      });
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    _timer = Timer(widget.debounce, () async {
      final token = CancelToken();
      _request = token;
      try {
        final page = await widget.client
            .autocomplete(query, limit: widget.limit, cancelToken: token);
        if (mounted && !token.isCancelled && generation == _generation) {
          setState(() {
            _results = page.data;
            _loading = false;
            _activeIndex = page.data.isEmpty ? -1 : 0;
          });
        }
      } on GhanaGeoCancelledException {
        // A newer query owns the visible state.
      } catch (_) {
        if (mounted && !token.isCancelled && generation == _generation) {
          setState(() {
            _results = const [];
            _activeIndex = -1;
            _error = 'Locations could not be loaded';
            _loading = false;
          });
        }
      }
    });
  }

  void _select(SearchResult result) {
    _controller
      ..text = result.name
      ..selection = TextSelection.collapsed(offset: result.name.length);
    setState(() {
      _results = const [];
      _activeIndex = -1;
    });
    widget.onSelected(result);
  }

  KeyEventResult _keyEvent(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent || _results.isEmpty) {
      return KeyEventResult.ignored;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
      setState(() =>
          _activeIndex = (_activeIndex + 1).clamp(0, _results.length - 1));
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
      setState(() =>
          _activeIndex = (_activeIndex - 1).clamp(0, _results.length - 1));
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.enter && _activeIndex >= 0) {
      _select(_results[_activeIndex]);
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.escape) {
      setState(() {
        _results = const [];
        _activeIndex = -1;
      });
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  @override
  Widget build(BuildContext context) => Focus(
        onKeyEvent: _keyEvent,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Semantics(
              textField: true,
              label: widget.label,
              child: TextField(
                controller: _controller,
                focusNode: _focusNode,
                onChanged: _changed,
                decoration: InputDecoration(
                  labelText: widget.label,
                  hintText: widget.hintText,
                  suffixIcon: _loading
                      ? const Padding(
                          padding: EdgeInsets.all(12),
                          child: CircularProgressIndicator(strokeWidth: 2))
                      : const Icon(Icons.search),
                  errorText: _error,
                ),
              ),
            ),
            Semantics(
              container: true,
              liveRegion: true,
              label: _error ??
                  (_loading
                      ? 'Loading location suggestions'
                      : '${_results.length} location suggestions'),
              child: const SizedBox(width: 1, height: 1),
            ),
            if (_results.isNotEmpty)
              Material(
                elevation: 8,
                borderRadius: BorderRadius.circular(16),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 320),
                  child: ListView.builder(
                    shrinkWrap: true,
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    itemCount: _results.length,
                    itemBuilder: (context, index) {
                      final result = _results[index];
                      return Semantics(
                        selected: index == _activeIndex,
                        button: true,
                        child: ListTile(
                          selected: index == _activeIndex,
                          title: Text(result.name),
                          subtitle: result.matchReason == null
                              ? null
                              : Text(result.matchReason!),
                          onTap: () => _select(result),
                        ),
                      );
                    },
                  ),
                ),
              ),
          ],
        ),
      );
}

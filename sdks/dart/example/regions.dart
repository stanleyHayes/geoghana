import 'package:ghanageo/ghanageo.dart';

Future<void> main() async {
  final client = GhanaGeoClient();
  try {
    final page = await client.regions(limit: 16);
    for (final region in page.data) {
      print('${region.name}: ${region.capital}');
    }
  } finally {
    client.close();
  }
}

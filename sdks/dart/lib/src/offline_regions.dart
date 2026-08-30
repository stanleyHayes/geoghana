import 'models.dart';

const offlineRegionsDatasetVersion = '2026.08.3-ulid';
const offlineRegionsPackageVersion = '0.1.0';

final offlineRegions = List<Region>.unmodifiable([
  _region('gh-region-ahafo', 'Ahafo', 'Goaso'),
  _region('gh-region-ashanti', 'Ashanti', 'Kumasi'),
  _region('gh-region-bono', 'Bono', 'Sunyani'),
  _region('gh-region-bono-east', 'Bono East', 'Techiman'),
  _region('gh-region-central', 'Central', 'Cape Coast'),
  _region('gh-region-eastern', 'Eastern', 'Koforidua'),
  _region('gh-region-greater-accra', 'Greater Accra', 'Accra'),
  _region('gh-region-north-east', 'North East', 'Nalerigu'),
  _region('gh-region-northern', 'Northern', 'Tamale'),
  _region('gh-region-oti', 'Oti', 'Dambai'),
  _region('gh-region-savannah', 'Savannah', 'Damongo'),
  _region('gh-region-upper-east', 'Upper East', 'Bolgatanga'),
  _region('gh-region-upper-west', 'Upper West', 'Wa'),
  _region('gh-region-volta', 'Volta', 'Ho'),
  _region('gh-region-western', 'Western', 'Sekondi-Takoradi'),
  _region('gh-region-western-north', 'Western North', 'Sefwi Wiawso'),
]);

Region _region(String id, String name, String capital) => Region(
      id: id,
      countryCode: 'GH',
      name: name,
      capital: capital,
      status: Status.active,
      verificationStatus: VerificationStatus.reference,
      provenance: const Provenance(sourceId: 'offline-regions'),
      datasetVersion: offlineRegionsDatasetVersion,
    );

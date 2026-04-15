import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/api/nube_client.dart';
import '../../agents/application/auth_provider.dart';
import '../models/store_app.dart';

/// Browse store apps — public catalog.
final storeAppsProvider =
    FutureProvider.family<StoreListResponse, StoreQuery>((ref, query) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) return const StoreListResponse(apps: [], total: 0, page: 1, limit: 20);
  return client.listStoreApps(
    query: query.search,
    category: query.category,
    sort: query.sort,
  );
});

/// Store categories.
final categoriesProvider = FutureProvider<List<String>>((ref) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) return [];
  return client.listCategories();
});

/// Store app detail.
final storeAppDetailProvider =
    FutureProvider.family<StoreAppDetail, String>((ref, id) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) throw Exception('Not authenticated');
  return client.getStoreApp(id);
});

/// Reviews for a store app.
final storeAppReviewsProvider =
    FutureProvider.family<List<AppReview>, String>((ref, appId) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) return [];
  return client.listStoreAppReviews(appId);
});

/// My store apps — authored by current user.
final myStoreAppsProvider = FutureProvider<List<StoreApp>>((ref) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) return [];
  return client.listMyStoreApps();
});

/// Query parameters for store browsing.
class StoreQuery {
  final String search;
  final String category;
  final String sort;

  const StoreQuery({
    this.search = '',
    this.category = '',
    this.sort = 'popular',
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is StoreQuery &&
          search == other.search &&
          category == other.category &&
          sort == other.sort;

  @override
  int get hashCode => Object.hash(search, category, sort);
}

import 'package:dio/dio.dart';
import '../../features/agents/models/agent.dart';
import '../../features/agents/models/session.dart';
import '../../features/store/models/store_app.dart';

class NubeClient {
  final Dio _dio;

  NubeClient({required String baseUrl, required String token})
      : _dio = Dio(BaseOptions(
          baseUrl: baseUrl,
          headers: {'Authorization': 'Bearer $token'},
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 30),
        ));

  // ── Health ──

  Future<bool> checkHealth() async {
    final res = await _dio.get('/health');
    return res.statusCode == 200;
  }

  // ── Users ──

  Future<Map<String, dynamic>> getMe() async {
    final res = await _dio.get('/users/me');
    return res.data as Map<String, dynamic>;
  }

  // ── Agents ──

  Future<List<Agent>> listAgents() async {
    final res = await _dio.get('/api/agents');
    final list = res.data as List<dynamic>;
    return list
        .map((e) => Agent.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  // ── Sessions ──

  Future<List<Session>> listSessions() async {
    final res = await _dio.get('/api/agents/sessions');
    final list = res.data as List<dynamic>;
    return list
        .map((e) => Session.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<Session> getSession(String id) async {
    final res = await _dio.get('/api/agents/sessions/$id');
    return Session.fromJson(res.data as Map<String, dynamic>);
  }

  // ── App CRUD ──

  Future<List<Map<String, dynamic>>> listApps() async {
    final res = await _dio.get('/apps');
    return (res.data as List).cast<Map<String, dynamic>>();
  }

  Future<Map<String, dynamic>> getApp(String name) async {
    final res = await _dio.get('/apps/$name');
    return res.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> createApp(Map<String, dynamic> data) async {
    final res = await _dio.post('/apps', data: data);
    return res.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> updateApp(
      String name, Map<String, dynamic> data) async {
    final res = await _dio.put('/apps/$name', data: data);
    return res.data as Map<String, dynamic>;
  }

  Future<void> deleteApp(String name) async {
    await _dio.delete('/apps/$name');
  }

  // ── Tool CRUD ──

  Future<List<Map<String, dynamic>>> listAppTools(String appName) async {
    final res = await _dio.get('/apps/$appName/tools');
    return (res.data as List).cast<Map<String, dynamic>>();
  }

  Future<Map<String, dynamic>> createTool(
      String appName, Map<String, dynamic> data) async {
    final res = await _dio.post('/apps/$appName/tools', data: data);
    return res.data as Map<String, dynamic>;
  }

  Future<void> updateTool(
      String appName, String toolName, Map<String, dynamic> data) async {
    await _dio.put('/apps/$appName/tools/$toolName', data: data);
  }

  Future<void> deleteTool(String appName, String toolName) async {
    await _dio.delete('/apps/$appName/tools/$toolName');
  }

  // ── Prompt CRUD ──

  Future<List<Map<String, dynamic>>> listAppPrompts(String appName) async {
    final res = await _dio.get('/apps/$appName/prompts');
    return (res.data as List).cast<Map<String, dynamic>>();
  }

  Future<Map<String, dynamic>> createPrompt(
      String appName, Map<String, dynamic> data) async {
    final res = await _dio.post('/apps/$appName/prompts', data: data);
    return res.data as Map<String, dynamic>;
  }

  Future<void> deletePrompt(String appName, String promptName) async {
    await _dio.delete('/apps/$appName/prompts/$promptName');
  }

  // ── App Install ──

  Future<Map<String, dynamic>> installApp(String appName) async {
    final res = await _dio.post('/apps/$appName/install');
    return res.data as Map<String, dynamic>;
  }

  // ── Call Tool (QA form mode) ──

  Future<Map<String, dynamic>> callTool(
      String toolName, Map<String, dynamic> params) async {
    final res = await _dio.post('/api/agents/tools/$toolName', data: params);
    return res.data as Map<String, dynamic>;
  }

  // ── App Store: Browse ──

  Future<StoreListResponse> listStoreApps({
    String? query,
    String? category,
    String sort = 'popular',
    int page = 1,
    int limit = 20,
  }) async {
    final params = <String, dynamic>{'sort': sort, 'page': page, 'limit': limit};
    if (query != null && query.isNotEmpty) params['q'] = query;
    if (category != null && category.isNotEmpty) params['category'] = category;
    final res = await _dio.get('/api/store/apps', queryParameters: params);
    final data = res.data as Map<String, dynamic>;
    final apps = (data['apps'] as List)
        .map((e) => StoreApp.fromSummary(e as Map<String, dynamic>))
        .toList();
    return StoreListResponse(apps: apps, total: data['total'] as int? ?? 0, page: data['page'] as int? ?? 1, limit: data['limit'] as int? ?? 20);
  }

  Future<StoreAppDetail> getStoreApp(String id) async {
    final res = await _dio.get('/api/store/apps/$id');
    final data = res.data as Map<String, dynamic>;
    return StoreAppDetail(
      app: StoreApp.fromJson(data['app'] as Map<String, dynamic>),
      installed: data['installed'] as bool? ?? false,
      installId: data['installId'] as String? ?? '',
    );
  }

  Future<List<String>> listCategories() async {
    final res = await _dio.get('/api/store/categories');
    return (res.data as List).cast<String>();
  }

  Future<Map<String, dynamic>> installStoreApp(String id, Map<String, String> settings) async {
    final res = await _dio.post('/api/store/apps/$id/install', data: {'settings': settings});
    return res.data as Map<String, dynamic>;
  }

  // ── App Store: Reviews ──

  Future<List<AppReview>> listStoreAppReviews(String appId) async {
    final res = await _dio.get('/api/store/apps/$appId/reviews');
    return (res.data as List).map((e) => AppReview.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<void> submitReview(String appId, int rating, String comment) async {
    await _dio.post('/api/store/apps/$appId/reviews', data: {'rating': rating, 'comment': comment});
  }

  Future<void> deleteReview(String appId) async {
    await _dio.delete('/api/store/apps/$appId/reviews');
  }

  // ── App Store: My Apps ──

  Future<List<StoreApp>> listMyStoreApps() async {
    final res = await _dio.get('/api/my/apps');
    return (res.data as List).map((e) => StoreApp.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<StoreApp> createStoreApp(Map<String, dynamic> data) async {
    final res = await _dio.post('/api/my/apps', data: data);
    return StoreApp.fromJson(res.data as Map<String, dynamic>);
  }

  Future<StoreApp> getMyStoreApp(String id) async {
    final res = await _dio.get('/api/my/apps/$id');
    return StoreApp.fromJson(res.data as Map<String, dynamic>);
  }

  Future<StoreApp> updateStoreApp(String id, Map<String, dynamic> data) async {
    final res = await _dio.put('/api/my/apps/$id', data: data);
    return StoreApp.fromJson(res.data as Map<String, dynamic>);
  }

  Future<void> deleteStoreApp(String id) async {
    await _dio.delete('/api/my/apps/$id');
  }

  Future<StoreApp> publishStoreApp(String id) async {
    final res = await _dio.post('/api/my/apps/$id/publish');
    return StoreApp.fromJson(res.data as Map<String, dynamic>);
  }

  Future<void> setStoreAppVisibility(String id, String visibility) async {
    await _dio.patch('/api/my/apps/$id/visibility', data: {'visibility': visibility});
  }

  // ── Store App: Tool/Prompt CRUD ──

  Future<void> addStoreTool(String appId, Map<String, dynamic> data) async {
    await _dio.post('/api/my/apps/$appId/tools', data: data);
  }

  Future<void> updateStoreTool(String appId, String toolName, Map<String, dynamic> data) async {
    await _dio.put('/api/my/apps/$appId/tools/$toolName', data: data);
  }

  Future<void> deleteStoreTool(String appId, String toolName) async {
    await _dio.delete('/api/my/apps/$appId/tools/$toolName');
  }

  Future<void> addStorePrompt(String appId, Map<String, dynamic> data) async {
    await _dio.post('/api/my/apps/$appId/prompts', data: data);
  }

  Future<void> updateStorePrompt(String appId, String promptName, Map<String, dynamic> data) async {
    await _dio.put('/api/my/apps/$appId/prompts/$promptName', data: data);
  }

  Future<void> deleteStorePrompt(String appId, String promptName) async {
    await _dio.delete('/api/my/apps/$appId/prompts/$promptName');
  }
}

// ── Response types ──

class StoreListResponse {
  final List<StoreApp> apps;
  final int total;
  final int page;
  final int limit;
  const StoreListResponse({required this.apps, required this.total, required this.page, required this.limit});
}

class StoreAppDetail {
  final StoreApp app;
  final bool installed;
  final String installId;
  const StoreAppDetail({required this.app, this.installed = false, this.installId = ''});
}

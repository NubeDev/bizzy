import 'package:dio/dio.dart';
import '../../features/agents/models/agent.dart';
import '../../features/agents/models/session.dart';

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
}

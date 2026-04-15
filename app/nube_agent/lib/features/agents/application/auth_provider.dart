import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/api/nube_client.dart';
import '../../../core/auth/auth_native.dart';
import '../../../core/auth/auth_repository.dart';
import '../../../core/auth/auth_web.dart';
import '../../../core/config/app_config.dart';

/// The auth repository — platform-specific implementation.
final authRepositoryProvider = Provider<AuthRepository>((ref) {
  if (AppConfig.usesBackendApi) {
    return WebAuthRepository();
  }
  return NativeAuthRepository();
});

/// Current auth state — null means not logged in.
final authProvider =
    AsyncNotifierProvider<AuthNotifier, AuthCredentials?>(AuthNotifier.new);

class AuthNotifier extends AsyncNotifier<AuthCredentials?> {
  @override
  Future<AuthCredentials?> build() async {
    final repo = ref.read(authRepositoryProvider);
    return repo.load();
  }

  Future<void> login(String serverUrl, String token) async {
    state = const AsyncLoading();
    try {
      final creds = AuthCredentials(serverUrl: serverUrl, token: token);

      // Validate credentials by calling /health then /users/me.
      final client = NubeClient(baseUrl: creds.serverUrl, token: creds.token);
      await client.checkHealth();
      await client.getMe();

      final repo = ref.read(authRepositoryProvider);
      await repo.save(creds);
      state = AsyncData(creds);
    } catch (e, st) {
      state = AsyncError(e, st);
    }
  }

  Future<void> logout() async {
    final repo = ref.read(authRepositoryProvider);
    await repo.clear();
    state = const AsyncData(null);
  }
}

/// NubeClient — only available when authenticated.
final nubeClientProvider = Provider<NubeClient?>((ref) {
  final auth = ref.watch(authProvider).valueOrNull;
  if (auth == null) return null;
  return NubeClient(baseUrl: auth.serverUrl, token: auth.token);
});

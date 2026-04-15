import 'package:shared_preferences/shared_preferences.dart';
import '../config/app_config.dart';
import 'auth_repository.dart';

/// Web auth — the Shelf backend proxies everything same-origin,
/// so we only need to remember that we're "logged in".
/// The server URL is always relative (empty string).
class WebAuthRepository implements AuthRepository {
  static const _keyToken = 'nube_token';

  @override
  Future<AuthCredentials?> load() async {
    final prefs = await SharedPreferences.getInstance();
    final token = prefs.getString(_keyToken);
    if (token == null) return null;
    return AuthCredentials(serverUrl: AppConfig.webBaseUrl, token: token);
  }

  @override
  Future<void> save(AuthCredentials credentials) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_keyToken, credentials.token);
  }

  @override
  Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_keyToken);
  }
}

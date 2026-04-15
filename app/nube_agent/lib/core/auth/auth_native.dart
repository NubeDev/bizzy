import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'auth_repository.dart';

class NativeAuthRepository implements AuthRepository {
  static const _keyUrl = 'nube_server_url';
  static const _keyToken = 'nube_token';
  final _storage = const FlutterSecureStorage();

  @override
  Future<AuthCredentials?> load() async {
    final url = await _storage.read(key: _keyUrl);
    final token = await _storage.read(key: _keyToken);
    if (url == null || token == null) return null;
    return AuthCredentials(serverUrl: url, token: token);
  }

  @override
  Future<void> save(AuthCredentials credentials) async {
    await _storage.write(key: _keyUrl, value: credentials.serverUrl);
    await _storage.write(key: _keyToken, value: credentials.token);
  }

  @override
  Future<void> clear() async {
    await _storage.delete(key: _keyUrl);
    await _storage.delete(key: _keyToken);
  }
}

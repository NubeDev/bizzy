/// Credentials stored/retrieved by the auth layer.
class AuthCredentials {
  final String serverUrl;
  final String token;

  const AuthCredentials({required this.serverUrl, required this.token});
}

/// Platform-agnostic auth storage interface.
abstract class AuthRepository {
  Future<AuthCredentials?> load();
  Future<void> save(AuthCredentials credentials);
  Future<void> clear();
}

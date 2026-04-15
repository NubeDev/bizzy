import 'package:flutter/foundation.dart' show kIsWeb;

class AppConfig {
  AppConfig._();

  /// True when running as a Flutter web app behind the Dart Shelf proxy.
  static bool get usesBackendApi => kIsWeb;

  /// Base URL used for REST + WS when running behind the web proxy.
  /// Relative paths work because the Shelf server is same-origin.
  static const String webBaseUrl = '';

  /// Default nube-server URL for native builds (user can override at login).
  static const String defaultServerUrl = 'http://localhost:8090';
}

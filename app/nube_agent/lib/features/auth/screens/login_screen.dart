import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../../core/config/app_config.dart';
import '../../agents/application/auth_provider.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _urlCtrl = TextEditingController(text: AppConfig.defaultServerUrl);
  final _tokenCtrl = TextEditingController();
  String? _error;

  @override
  void dispose() {
    _urlCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    setState(() => _error = null);
    final url = _urlCtrl.text.trim();
    final token = _tokenCtrl.text.trim();
    if (url.isEmpty || token.isEmpty) {
      setState(() => _error = 'Server URL and token are required.');
      return;
    }
    await ref.read(authProvider.notifier).login(url, token);
    final state = ref.read(authProvider);
    if (state.hasError) {
      setState(() => _error = 'Login failed: ${state.error}');
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);
    final isLoading = ref.watch(authProvider).isLoading;

    return Scaffold(
      backgroundColor: colors.bg,
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 380),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // Logo
                Center(
                  child: Container(
                    width: 56,
                    height: 56,
                    decoration: BoxDecoration(
                      color: RubixTokens.accentCool,
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: const Icon(
                      LucideIcons.bot,
                      size: 28,
                      color: Colors.black,
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text(
                  'Welcome back',
                  style: colors.inter(
                    fontSize: 24,
                    fontWeight: FontWeight.w600,
                  ),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 6),
                Text(
                  'Connect to your nube-server',
                  style: colors.inter(
                    fontSize: 14,
                    color: colors.textMuted,
                  ),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 32),

                // Server URL
                _Label(text: 'Server URL', colors: colors),
                const SizedBox(height: 6),
                _TextField(
                  controller: _urlCtrl,
                  placeholder: 'http://localhost:8090',
                  enabled: !isLoading,
                  colors: colors,
                ),
                const SizedBox(height: 16),

                // Token
                _Label(text: 'Token', colors: colors),
                const SizedBox(height: 6),
                _TextField(
                  controller: _tokenCtrl,
                  placeholder: 'Paste your bearer token',
                  obscureText: true,
                  enabled: !isLoading,
                  colors: colors,
                ),

                // Error
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: RubixTokens.statusError.withValues(alpha: 0.08),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Text(
                      _error!,
                      style: colors.inter(
                        fontSize: 13,
                        color: RubixTokens.statusError,
                      ),
                    ),
                  ),
                ],

                const SizedBox(height: 24),

                // Submit
                SizedBox(
                  height: 44,
                  child: ElevatedButton(
                    onPressed: isLoading ? null : _login,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: RubixTokens.accentCool,
                      foregroundColor: Colors.black,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10),
                      ),
                      elevation: 0,
                      textStyle: colors.inter(
                        fontSize: 15,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                    child: isLoading
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Colors.black,
                            ),
                          )
                        : const Text('Continue'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _Label extends StatelessWidget {
  final String text;
  final RubixColors colors;

  const _Label({required this.text, required this.colors});

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: colors.inter(
        fontSize: 13,
        fontWeight: FontWeight.w500,
        color: colors.textSecondary,
      ),
    );
  }
}

class _TextField extends StatelessWidget {
  final TextEditingController controller;
  final String placeholder;
  final bool obscureText;
  final bool enabled;
  final RubixColors colors;

  const _TextField({
    required this.controller,
    required this.placeholder,
    this.obscureText = false,
    this.enabled = true,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: colors.surfaceBright,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: colors.border, width: 1),
      ),
      child: TextField(
        controller: controller,
        obscureText: obscureText,
        enabled: enabled,
        style: colors.inter(fontSize: 14),
        cursorColor: RubixTokens.accentCool,
        decoration: InputDecoration(
          hintText: placeholder,
          hintStyle: colors.inter(fontSize: 14, color: colors.textMuted),
          border: InputBorder.none,
          contentPadding:
              const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          isDense: true,
        ),
      ),
    );
  }
}

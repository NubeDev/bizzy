import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../../core/api/nube_client.dart';
import '../../agents/application/auth_provider.dart';

final _appsProvider = FutureProvider<List<Map<String, dynamic>>>((ref) async {
  final auth = ref.watch(authProvider).valueOrNull;
  if (auth == null) return [];
  final client = NubeClient(baseUrl: auth.serverUrl, token: auth.token);
  return client.listApps();
});

class AdminScreen extends ConsumerWidget {
  const AdminScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final tokens = RubixTokens.of(context);
    final apps = ref.watch(_appsProvider);

    return Scaffold(
      backgroundColor: tokens.bg,
      body: CustomScrollView(
        slivers: [
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(24, 24, 24, 16),
              child: Row(
                children: [
                  Text(
                    'Apps',
                    style: tokens.heading(fontSize: 24),
                  ),
                  const Spacer(),
                  RubixButton.icon(
                    icon: LucideIcons.refreshCw,
                    onPressed: () => ref.invalidate(_appsProvider),
                  ),
                ],
              ),
            ),
          ),
          apps.when(
            data: (list) => SliverPadding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              sliver: SliverList.separated(
                itemCount: list.length,
                separatorBuilder: (_, __) => const SizedBox(height: 8),
                itemBuilder: (context, index) {
                  final app = list[index];
                  final name = app['name'] as String? ?? '';
                  final desc = app['description'] as String? ?? '';
                  final version = app['version'] as String? ?? '';
                  final hasTools = app['hasTools'] == true;
                  final hasPrompts = app['hasPrompts'] == true;

                  return RubixCard(
                    onTap: () => _showAppDetail(context, ref, app),
                    child: Row(
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Text(name, style: tokens.inter(
                                    fontSize: 15,
                                    fontWeight: FontWeight.w600,
                                  )),
                                  const SizedBox(width: 8),
                                  Text('v$version', style: tokens.mono(
                                    fontSize: 12,
                                    color: tokens.textMuted,
                                  )),
                                ],
                              ),
                              if (desc.isNotEmpty) ...[
                                const SizedBox(height: 4),
                                Text(desc, style: tokens.inter(
                                  fontSize: 13,
                                  color: tokens.textSecondary,
                                )),
                              ],
                              const SizedBox(height: 6),
                              Row(
                                children: [
                                  if (hasTools)
                                    _Badge(label: 'tools', colors: tokens),
                                  if (hasPrompts)
                                    _Badge(label: 'prompts', colors: tokens),
                                ],
                              ),
                            ],
                          ),
                        ),
                        RubixButton.icon(
                          icon: LucideIcons.trash2,
                          onPressed: () => _deleteApp(context, ref, name),
                        ),
                      ],
                    ),
                  );
                },
              ),
            ),
            loading: () => const SliverFillRemaining(
              child: Center(child: RubixLoader()),
            ),
            error: (e, _) => SliverFillRemaining(
              child: RubixErrorState(
                message: 'Failed to load apps',
                detail: e.toString(),
                onRetry: () => ref.invalidate(_appsProvider),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showAppDetail(
      BuildContext context, WidgetRef ref, Map<String, dynamic> app) async {
    final auth = ref.read(authProvider).valueOrNull;
    if (auth == null) return;

    final client = NubeClient(baseUrl: auth.serverUrl, token: auth.token);
    final name = app['name'] as String;

    final tools = await client.listAppTools(name);
    final prompts = await client.listAppPrompts(name);

    if (!context.mounted) return;

    final tokens = RubixTokens.of(context);

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: tokens.surface,
        title: Text(name, style: tokens.heading(fontSize: 18)),
        content: SizedBox(
          width: 400,
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                if (tools.isNotEmpty) ...[
                  Text('Tools', style: tokens.inter(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: tokens.textSecondary,
                  )),
                  const SizedBox(height: 8),
                  for (final t in tools)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 4),
                      child: Row(
                        children: [
                          Icon(LucideIcons.wrench, size: 14,
                              color: tokens.textMuted),
                          const SizedBox(width: 8),
                          Text(
                            t['name'] as String? ?? '',
                            style: tokens.mono(fontSize: 13),
                          ),
                          if (t['mode'] == 'qa') ...[
                            const SizedBox(width: 6),
                            _Badge(label: 'QA', colors: tokens),
                          ],
                        ],
                      ),
                    ),
                  const SizedBox(height: 16),
                ],
                if (prompts.isNotEmpty) ...[
                  Text('Prompts', style: tokens.inter(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: tokens.textSecondary,
                  )),
                  const SizedBox(height: 8),
                  for (final p in prompts)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 4),
                      child: Row(
                        children: [
                          Icon(LucideIcons.fileText, size: 14,
                              color: tokens.textMuted),
                          const SizedBox(width: 8),
                          Text(
                            p['name'] as String? ?? '',
                            style: tokens.mono(fontSize: 13),
                          ),
                        ],
                      ),
                    ),
                ],
                if (tools.isEmpty && prompts.isEmpty)
                  Text('No tools or prompts.',
                      style: tokens.inter(color: tokens.textMuted)),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Close'),
          ),
        ],
      ),
    );
  }

  void _deleteApp(
      BuildContext context, WidgetRef ref, String name) async {
    final confirmed = await showRubixConfirm(
      context,
      title: 'Delete $name?',
      description: 'This will remove the app and all its tools and prompts.',
      confirmLabel: 'Delete',
      destructive: true,
    );

    if (confirmed != true) return;

    final auth = ref.read(authProvider).valueOrNull;
    if (auth == null) return;

    final client = NubeClient(baseUrl: auth.serverUrl, token: auth.token);
    await client.deleteApp(name);
    ref.invalidate(_appsProvider);
  }
}

class _Badge extends StatelessWidget {
  final String label;
  final RubixColors colors;

  const _Badge({required this.label, required this.colors});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(right: 6),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: colors.surfaceBright,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: colors.mono(fontSize: 10, color: colors.textMuted),
      ),
    );
  }
}

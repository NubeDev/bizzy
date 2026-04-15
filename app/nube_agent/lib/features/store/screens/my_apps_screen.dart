import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../agents/application/auth_provider.dart';
import '../application/store_provider.dart';
import '../models/store_app.dart';

class MyAppsScreen extends ConsumerWidget {
  const MyAppsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = RubixTokens.of(context);
    final myAppsAsync = ref.watch(myStoreAppsProvider);

    return Scaffold(
      backgroundColor: colors.bg,
      body: CustomScrollView(
        slivers: [
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(24, 24, 24, 16),
              child: Row(
                children: [
                  RubixButton.icon(
                    icon: LucideIcons.arrowLeft,
                    onPressed: () => context.go('/store'),
                  ),
                  const SizedBox(width: 12),
                  Text('My Apps', style: colors.heading(fontSize: 24)),
                  const Spacer(),
                  ElevatedButton.icon(
                    onPressed: () => _showCreateDialog(context, ref),
                    icon: const Icon(LucideIcons.plus, size: 16),
                    label: const Text('Create App'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: RubixTokens.accentCool,
                      foregroundColor: Colors.black,
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8)),
                    ),
                  ),
                ],
              ),
            ),
          ),
          myAppsAsync.when(
            data: (apps) {
              if (apps.isEmpty) {
                return SliverFillRemaining(
                  child: Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(LucideIcons.packageOpen,
                            size: 48, color: colors.textMuted),
                        const SizedBox(height: 16),
                        Text('No apps yet',
                            style: colors.inter(
                                fontSize: 16, fontWeight: FontWeight.w600)),
                        const SizedBox(height: 4),
                        Text('Create your first app to get started.',
                            style: colors.inter(
                                fontSize: 13, color: colors.textMuted)),
                      ],
                    ),
                  ),
                );
              }
              return SliverPadding(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                sliver: SliverList.separated(
                  itemCount: apps.length,
                  separatorBuilder: (_, __) => const SizedBox(height: 8),
                  itemBuilder: (context, index) =>
                      _MyAppCard(app: apps[index], colors: colors),
                ),
              );
            },
            loading: () => const SliverFillRemaining(
                child: Center(child: RubixLoader())),
            error: (e, _) => SliverFillRemaining(
              child: RubixErrorState(
                message: 'Failed to load apps',
                detail: e.toString(),
                onRetry: () => ref.invalidate(myStoreAppsProvider),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showCreateDialog(BuildContext context, WidgetRef ref) {
    final nameCtrl = TextEditingController();
    final displayCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    final colors = RubixTokens.of(context);

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: colors.surface,
        title: Text('Create App', style: colors.heading(fontSize: 18)),
        content: SizedBox(
          width: 400,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: nameCtrl,
                style: colors.inter(fontSize: 14),
                decoration: InputDecoration(
                  labelText: 'Name (slug)',
                  hintText: 'my-cool-tool',
                  hintStyle:
                      colors.inter(fontSize: 14, color: colors.textMuted),
                  border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: displayCtrl,
                style: colors.inter(fontSize: 14),
                decoration: InputDecoration(
                  labelText: 'Display Name',
                  hintText: 'My Cool Tool',
                  hintStyle:
                      colors.inter(fontSize: 14, color: colors.textMuted),
                  border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: descCtrl,
                maxLines: 2,
                style: colors.inter(fontSize: 14),
                decoration: InputDecoration(
                  labelText: 'Description',
                  border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () async {
              final client = ref.read(nubeClientProvider);
              if (client == null) return;

              final app = await client.createStoreApp({
                'name': nameCtrl.text.trim(),
                'displayName': displayCtrl.text.trim(),
                'description': descCtrl.text.trim(),
              });
              ref.invalidate(myStoreAppsProvider);
              if (context.mounted) {
                Navigator.pop(context);
                context.go('/my-apps/${app.id}');
              }
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: RubixTokens.accentCool,
              foregroundColor: Colors.black,
            ),
            child: const Text('Create'),
          ),
        ],
      ),
    );
  }
}

class _MyAppCard extends ConsumerWidget {
  final StoreApp app;
  final RubixColors colors;

  const _MyAppCard({required this.app, required this.colors});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return RubixCard(
      onTap: () => context.go('/my-apps/${app.id}'),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      app.displayName.isNotEmpty ? app.displayName : app.name,
                      style: colors.inter(
                          fontSize: 15, fontWeight: FontWeight.w600),
                    ),
                    const SizedBox(width: 8),
                    Text('v${app.version}',
                        style:
                            colors.mono(fontSize: 12, color: colors.textMuted)),
                    const SizedBox(width: 8),
                    _VisibilityBadge(
                        visibility: app.visibility, colors: colors),
                  ],
                ),
                if (app.description.isNotEmpty) ...[
                  const SizedBox(height: 4),
                  Text(app.description,
                      style: colors.inter(
                          fontSize: 13, color: colors.textSecondary)),
                ],
                const SizedBox(height: 6),
                Row(
                  children: [
                    Text('${app.tools.length} tools',
                        style:
                            colors.mono(fontSize: 11, color: colors.textMuted)),
                    const SizedBox(width: 10),
                    Text('${app.prompts.length} prompts',
                        style:
                            colors.mono(fontSize: 11, color: colors.textMuted)),
                    const SizedBox(width: 10),
                    Text('${app.installCount} installs',
                        style:
                            colors.mono(fontSize: 11, color: colors.textMuted)),
                  ],
                ),
              ],
            ),
          ),
          RubixButton.icon(
            icon: LucideIcons.trash2,
            onPressed: () => _delete(context, ref),
          ),
        ],
      ),
    );
  }

  void _delete(BuildContext context, WidgetRef ref) async {
    final confirmed = await showRubixConfirm(
      context,
      title: 'Delete ${app.name}?',
      description: 'This will permanently delete the app and all its data.',
      confirmLabel: 'Delete',
      destructive: true,
    );
    if (confirmed != true) return;
    final client = ref.read(nubeClientProvider);
    if (client == null) return;
    await client.deleteStoreApp(app.id);
    ref.invalidate(myStoreAppsProvider);
  }
}

class _VisibilityBadge extends StatelessWidget {
  final String visibility;
  final RubixColors colors;

  const _VisibilityBadge({required this.visibility, required this.colors});

  @override
  Widget build(BuildContext context) {
    Color badgeColor;
    switch (visibility) {
      case 'public':
        badgeColor = const Color(0xFF10B981);
        break;
      case 'unlisted':
        badgeColor = const Color(0xFFF59E0B);
        break;
      case 'shared':
        badgeColor = const Color(0xFF3B82F6);
        break;
      default:
        badgeColor = colors.textMuted;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: badgeColor.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        visibility,
        style: colors.mono(fontSize: 10, color: badgeColor),
      ),
    );
  }
}

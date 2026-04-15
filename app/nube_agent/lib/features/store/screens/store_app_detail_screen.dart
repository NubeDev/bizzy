import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../../core/api/nube_client.dart';
import '../../agents/application/auth_provider.dart';
import '../application/store_provider.dart';

class StoreAppDetailScreen extends ConsumerWidget {
  final String appId;

  const StoreAppDetailScreen({super.key, required this.appId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = RubixTokens.of(context);
    final detailAsync = ref.watch(storeAppDetailProvider(appId));

    return Scaffold(
      backgroundColor: colors.bg,
      body: detailAsync.when(
        loading: () => const Center(child: RubixLoader()),
        error: (e, _) => Center(
          child: RubixErrorState(
            message: 'Failed to load app',
            detail: e.toString(),
            onRetry: () => ref.invalidate(storeAppDetailProvider(appId)),
          ),
        ),
        data: (detail) => _DetailBody(detail: detail, colors: colors),
      ),
    );
  }
}

class _DetailBody extends ConsumerStatefulWidget {
  final StoreAppDetail detail;
  final RubixColors colors;

  const _DetailBody({required this.detail, required this.colors});

  @override
  ConsumerState<_DetailBody> createState() => _DetailBodyState();
}

class _DetailBodyState extends ConsumerState<_DetailBody> {
  bool _installing = false;
  late bool _installed;

  @override
  void initState() {
    super.initState();
    _installed = widget.detail.installed;
  }

  @override
  Widget build(BuildContext context) {
    final app = widget.detail.app;
    final colors = widget.colors;

    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 700),
        child: ListView(
          padding: const EdgeInsets.all(24),
          children: [
            // Back + title
            Row(
              children: [
                RubixButton.icon(
                  icon: LucideIcons.arrowLeft,
                  onPressed: () => context.go('/store'),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    app.displayName.isNotEmpty ? app.displayName : app.name,
                    style: colors.heading(fontSize: 22),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              'by ${app.authorName}  |  v${app.version}  |  ${app.category}',
              style: colors.inter(fontSize: 13, color: colors.textMuted),
            ),
            const SizedBox(height: 20),

            // Stats row
            Row(
              children: [
                _Stat(icon: LucideIcons.download, label: '${app.installCount}', colors: colors),
                const SizedBox(width: 20),
                _Stat(icon: LucideIcons.star, label: app.avgRating > 0 ? app.avgRating.toStringAsFixed(1) : '--', colors: colors),
                const SizedBox(width: 20),
                _Stat(icon: LucideIcons.messageCircle, label: '${app.reviewCount}', colors: colors),
                const Spacer(),
                if (_installed)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                    decoration: BoxDecoration(
                      color: const Color(0xFF10B981).withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(LucideIcons.check, size: 14, color: Color(0xFF10B981)),
                        const SizedBox(width: 6),
                        Text('Installed', style: colors.inter(
                          fontSize: 13, fontWeight: FontWeight.w600,
                          color: const Color(0xFF10B981),
                        )),
                      ],
                    ),
                  )
                else
                  ElevatedButton.icon(
                    onPressed: _installing ? null : _install,
                    icon: _installing
                        ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Icon(LucideIcons.download, size: 14),
                    label: Text(_installing ? 'Installing...' : 'Install'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: RubixTokens.accentCool,
                      foregroundColor: Colors.black,
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                    ),
                  ),
              ],
            ),

            const SizedBox(height: 24),

            // Description
            if (app.description.isNotEmpty) ...[
              Text(app.description, style: colors.inter(fontSize: 14, color: colors.textSecondary, height: 1.5)),
              const SizedBox(height: 16),
            ],

            if (app.longDescription.isNotEmpty) ...[
              Text(app.longDescription, style: colors.inter(fontSize: 14, color: colors.textSecondary, height: 1.5)),
              const SizedBox(height: 24),
            ],

            // Tags
            if (app.tags.isNotEmpty) ...[
              Wrap(
                spacing: 6,
                runSpacing: 6,
                children: app.tags
                    .map((t) => Container(
                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                          decoration: BoxDecoration(
                            color: colors.surfaceBright,
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: Text(t, style: colors.mono(fontSize: 11, color: colors.textMuted)),
                        ))
                    .toList(),
              ),
              const SizedBox(height: 24),
            ],

            // Tools
            if (app.tools.isNotEmpty) ...[
              Text('Tools', style: colors.inter(fontSize: 16, fontWeight: FontWeight.w600)),
              const SizedBox(height: 8),
              for (final tool in app.tools)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: RubixCard(
                    child: Row(
                      children: [
                        Icon(LucideIcons.wrench, size: 16, color: colors.textMuted),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(tool.name, style: colors.mono(fontSize: 13, fontWeight: FontWeight.w600)),
                              if (tool.description.isNotEmpty)
                                Text(tool.description, style: colors.inter(fontSize: 12, color: colors.textSecondary)),
                            ],
                          ),
                        ),
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: colors.surfaceBright,
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(tool.toolClass, style: colors.mono(fontSize: 10, color: colors.textMuted)),
                        ),
                      ],
                    ),
                  ),
                ),
              const SizedBox(height: 16),
            ],

            // Prompts
            if (app.prompts.isNotEmpty) ...[
              Text('Prompts', style: colors.inter(fontSize: 16, fontWeight: FontWeight.w600)),
              const SizedBox(height: 8),
              for (final prompt in app.prompts)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: RubixCard(
                    child: Row(
                      children: [
                        Icon(LucideIcons.fileText, size: 16, color: colors.textMuted),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(prompt.name, style: colors.mono(fontSize: 13, fontWeight: FontWeight.w600)),
                              if (prompt.description.isNotEmpty)
                                Text(prompt.description, style: colors.inter(fontSize: 12, color: colors.textSecondary)),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              const SizedBox(height: 16),
            ],

            // Reviews section
            _ReviewsSection(appId: app.id, colors: colors),
          ],
        ),
      ),
    );
  }

  Future<void> _install() async {
    setState(() => _installing = true);
    try {
      final client = ref.read(nubeClientProvider);
      if (client == null) return;
      await client.installStoreApp(widget.detail.app.id, {});
      setState(() {
        _installed = true;
        _installing = false;
      });
    } catch (e) {
      setState(() => _installing = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Install failed: $e')),
        );
      }
    }
  }
}

class _Stat extends StatelessWidget {
  final IconData icon;
  final String label;
  final RubixColors colors;

  const _Stat({required this.icon, required this.label, required this.colors});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 14, color: colors.textMuted),
        const SizedBox(width: 4),
        Text(label, style: colors.mono(fontSize: 13, color: colors.textSecondary)),
      ],
    );
  }
}

// ── Reviews section ──

class _ReviewsSection extends ConsumerWidget {
  final String appId;
  final RubixColors colors;

  const _ReviewsSection({required this.appId, required this.colors});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final reviewsAsync = ref.watch(storeAppReviewsProvider(appId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text('Reviews', style: colors.inter(fontSize: 16, fontWeight: FontWeight.w600)),
            const Spacer(),
            RubixButton.icon(
              icon: LucideIcons.plus,
              onPressed: () => _showReviewDialog(context, ref),
            ),
          ],
        ),
        const SizedBox(height: 8),
        reviewsAsync.when(
          data: (reviews) {
            if (reviews.isEmpty) {
              return Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text('No reviews yet.', style: colors.inter(fontSize: 13, color: colors.textMuted)),
              );
            }
            return Column(
              children: reviews.map((r) => Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: RubixCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Text(r.userName, style: colors.inter(fontSize: 13, fontWeight: FontWeight.w600)),
                          const Spacer(),
                          Row(
                            children: List.generate(5, (i) => Icon(
                              i < r.rating ? LucideIcons.star : LucideIcons.star,
                              size: 12,
                              color: i < r.rating ? const Color(0xFFFBBF24) : colors.textMuted,
                            )),
                          ),
                        ],
                      ),
                      if (r.comment.isNotEmpty) ...[
                        const SizedBox(height: 4),
                        Text(r.comment, style: colors.inter(fontSize: 13, color: colors.textSecondary)),
                      ],
                    ],
                  ),
                ),
              )).toList(),
            );
          },
          loading: () => const Padding(
            padding: EdgeInsets.only(top: 16),
            child: Center(child: RubixLoader()),
          ),
          error: (e, _) => Text('Failed to load reviews', style: colors.inter(color: colors.textMuted)),
        ),
      ],
    );
  }

  void _showReviewDialog(BuildContext context, WidgetRef ref) {
    int rating = 5;
    final commentCtrl = TextEditingController();
    final colors = RubixTokens.of(context);

    showDialog(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setState) => AlertDialog(
          backgroundColor: colors.surface,
          title: Text('Write a Review', style: colors.heading(fontSize: 18)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: List.generate(5, (i) => GestureDetector(
                  onTap: () => setState(() => rating = i + 1),
                  child: Padding(
                    padding: const EdgeInsets.all(4),
                    child: Icon(
                      LucideIcons.star,
                      size: 24,
                      color: i < rating ? const Color(0xFFFBBF24) : colors.textMuted,
                    ),
                  ),
                )),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: commentCtrl,
                maxLines: 3,
                style: colors.inter(fontSize: 14),
                decoration: InputDecoration(
                  hintText: 'Optional comment...',
                  hintStyle: colors.inter(fontSize: 14, color: colors.textMuted),
                  border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
                ),
              ),
            ],
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
                await client.submitReview(appId, rating, commentCtrl.text);
                ref.invalidate(storeAppReviewsProvider(appId));
                ref.invalidate(storeAppDetailProvider(appId));
                if (context.mounted) Navigator.pop(context);
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: RubixTokens.accentCool,
                foregroundColor: Colors.black,
              ),
              child: const Text('Submit'),
            ),
          ],
        ),
      ),
    );
  }
}

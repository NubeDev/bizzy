import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../application/store_provider.dart';
import '../models/store_app.dart';

/// Icon + gradient for a category.
class _CatTheme {
  final IconData icon;
  final List<Color> gradient;
  const _CatTheme(this.icon, this.gradient);
}

final _catThemes = <String, _CatTheme>{
  'iot-devices': _CatTheme(LucideIcons.cpu, [const Color(0xFF34D399), const Color(0xFF10B981)]),
  'analytics': _CatTheme(LucideIcons.barChart2, [const Color(0xFF38BDF8), const Color(0xFF3B82F6)]),
  'devops': _CatTheme(LucideIcons.terminal, [const Color(0xFF71D5E3), const Color(0xFF309EAB)]),
  'marketing': _CatTheme(LucideIcons.megaphone, [const Color(0xFFE879F9), const Color(0xFFA855F7)]),
  'design': _CatTheme(LucideIcons.palette, [const Color(0xFFFB923C), const Color(0xFFF97316)]),
  'utilities': _CatTheme(LucideIcons.wrench, [const Color(0xFFA78BFA), const Color(0xFF8B5CF6)]),
  'integrations': _CatTheme(LucideIcons.plug, [const Color(0xFFFBBF24), const Color(0xFFF59E0B)]),
  'automation': _CatTheme(LucideIcons.zap, [const Color(0xFFF472B6), const Color(0xFFEC4899)]),
};

_CatTheme _themeFor(String category) =>
    _catThemes[category] ??
    _CatTheme(LucideIcons.box, [const Color(0xFF71D5E3), const Color(0xFF309EAB)]);

class StoreScreen extends ConsumerStatefulWidget {
  const StoreScreen({super.key});

  @override
  ConsumerState<StoreScreen> createState() => _StoreScreenState();
}

class _StoreScreenState extends ConsumerState<StoreScreen> {
  String _search = '';
  String _category = '';
  String _sort = 'popular';

  StoreQuery get _query =>
      StoreQuery(search: _search, category: _category, sort: _sort);

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);
    final storeAsync = ref.watch(storeAppsProvider(_query));

    return Scaffold(
      backgroundColor: colors.bg,
      body: CustomScrollView(
        slivers: [
          // Header
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text('App Store', style: colors.heading(fontSize: 24)),
                      const Spacer(),
                      RubixButton.icon(
                        icon: LucideIcons.plus,
                        onPressed: () => context.go('/my-apps'),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  // Search bar
                  _SearchBar(
                    onChanged: (v) => setState(() => _search = v),
                    colors: colors,
                  ),
                  const SizedBox(height: 12),
                  // Filters
                  _FilterRow(
                    category: _category,
                    sort: _sort,
                    onCategoryChanged: (v) => setState(() => _category = v),
                    onSortChanged: (v) => setState(() => _sort = v),
                    colors: colors,
                  ),
                  const SizedBox(height: 16),
                ],
              ),
            ),
          ),

          // Results
          storeAsync.when(
            data: (result) {
              if (result.apps.isEmpty) {
                return SliverFillRemaining(
                  child: Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(LucideIcons.store, size: 48, color: colors.textMuted),
                        const SizedBox(height: 16),
                        Text('No apps found',
                            style: colors.inter(fontSize: 16, fontWeight: FontWeight.w600)),
                        const SizedBox(height: 4),
                        Text('Try a different search or category.',
                            style: colors.inter(fontSize: 13, color: colors.textMuted)),
                      ],
                    ),
                  ),
                );
              }

              return SliverPadding(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                sliver: SliverGrid(
                  gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                    maxCrossAxisExtent: 360,
                    mainAxisExtent: 190,
                    crossAxisSpacing: 12,
                    mainAxisSpacing: 12,
                  ),
                  delegate: SliverChildBuilderDelegate(
                    (context, index) => _StoreAppCard(
                      app: result.apps[index],
                      colors: colors,
                      onTap: () =>
                          context.go('/store/${result.apps[index].id}'),
                    ),
                    childCount: result.apps.length,
                  ),
                ),
              );
            },
            loading: () =>
                const SliverFillRemaining(child: Center(child: RubixLoader())),
            error: (e, _) => SliverFillRemaining(
              child: RubixErrorState(
                message: 'Failed to load store',
                detail: e.toString(),
                onRetry: () => ref.invalidate(storeAppsProvider(_query)),
              ),
            ),
          ),

          const SliverToBoxAdapter(child: SizedBox(height: 24)),
        ],
      ),
    );
  }
}

// ── Search bar ──

class _SearchBar extends StatelessWidget {
  final ValueChanged<String> onChanged;
  final RubixColors colors;

  const _SearchBar({required this.onChanged, required this.colors});

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: colors.surface,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: colors.border, width: 0.5),
      ),
      child: TextField(
        onChanged: onChanged,
        style: colors.inter(fontSize: 14),
        decoration: InputDecoration(
          hintText: 'Search apps...',
          hintStyle: colors.inter(fontSize: 14, color: colors.textMuted),
          prefixIcon: Icon(LucideIcons.search, size: 18, color: colors.textMuted),
          border: InputBorder.none,
          contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
        ),
      ),
    );
  }
}

// ── Filter row ──

class _FilterRow extends StatelessWidget {
  final String category;
  final String sort;
  final ValueChanged<String> onCategoryChanged;
  final ValueChanged<String> onSortChanged;
  final RubixColors colors;

  const _FilterRow({
    required this.category,
    required this.sort,
    required this.onCategoryChanged,
    required this.onSortChanged,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    final cats = ['', 'iot-devices', 'analytics', 'devops', 'marketing',
        'design', 'utilities', 'integrations', 'automation'];
    final sorts = ['popular', 'recent', 'rating', 'name'];

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          // Category chips
          for (final cat in cats)
            Padding(
              padding: const EdgeInsets.only(right: 6),
              child: _Chip(
                label: cat.isEmpty ? 'All' : cat,
                selected: category == cat,
                onTap: () => onCategoryChanged(cat),
                colors: colors,
              ),
            ),
          const SizedBox(width: 12),
          // Sort dropdown
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            decoration: BoxDecoration(
              color: colors.surface,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: colors.border, width: 0.5),
            ),
            child: DropdownButtonHideUnderline(
              child: DropdownButton<String>(
                value: sort,
                isDense: true,
                style: colors.inter(fontSize: 12, color: colors.textSecondary),
                dropdownColor: colors.surface,
                items: sorts.map((s) => DropdownMenuItem(
                  value: s,
                  child: Text(s[0].toUpperCase() + s.substring(1)),
                )).toList(),
                onChanged: (v) { if (v != null) onSortChanged(v); },
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;
  final RubixColors colors;

  const _Chip({
    required this.label,
    required this.selected,
    required this.onTap,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: selected ? RubixTokens.accentCool.withValues(alpha: 0.15) : colors.surface,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: selected ? RubixTokens.accentCool : colors.border,
            width: 0.5,
          ),
        ),
        child: Text(
          label,
          style: colors.inter(
            fontSize: 12,
            fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
            color: selected ? RubixTokens.accentCool : colors.textSecondary,
          ),
        ),
      ),
    );
  }
}

// ── Store app card ──

class _StoreAppCard extends StatelessWidget {
  final StoreApp app;
  final RubixColors colors;
  final VoidCallback onTap;

  const _StoreAppCard({
    required this.app,
    required this.colors,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = _themeFor(app.category);

    return Material(
      color: colors.surfaceBright,
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        borderRadius: BorderRadius.circular(14),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: colors.border, width: 0.5),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  // Icon
                  Container(
                    width: 40,
                    height: 40,
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        colors: app.color.isNotEmpty
                            ? [_parseColor(app.color), _parseColor(app.color).withValues(alpha: 0.7)]
                            : theme.gradient,
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Icon(theme.icon, size: 20, color: Colors.white),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          app.displayName.isNotEmpty ? app.displayName : app.name,
                          style: colors.inter(fontSize: 15, fontWeight: FontWeight.w600),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        Text(
                          app.authorName,
                          style: colors.inter(fontSize: 12, color: colors.textMuted),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              Expanded(
                child: Text(
                  app.description,
                  style: colors.inter(fontSize: 13, color: colors.textSecondary, height: 1.4),
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(height: 8),
              // Bottom stats
              Row(
                children: [
                  if (app.avgRating > 0) ...[
                    Icon(LucideIcons.star, size: 12, color: const Color(0xFFFBBF24)),
                    const SizedBox(width: 3),
                    Text(app.avgRating.toStringAsFixed(1),
                        style: colors.mono(fontSize: 11, color: colors.textMuted)),
                    const SizedBox(width: 10),
                  ],
                  Icon(LucideIcons.download, size: 12, color: colors.textMuted),
                  const SizedBox(width: 3),
                  Text('${app.installCount}',
                      style: colors.mono(fontSize: 11, color: colors.textMuted)),
                  const Spacer(),
                  if (app.toolCount > 0)
                    Text('${app.toolCount} tools',
                        style: colors.mono(fontSize: 11, color: colors.textMuted)),
                  if (app.toolCount > 0 && app.promptCount > 0)
                    Text(' + ', style: colors.mono(fontSize: 11, color: colors.textMuted)),
                  if (app.promptCount > 0)
                    Text('${app.promptCount} prompts',
                        style: colors.mono(fontSize: 11, color: colors.textMuted)),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

Color _parseColor(String hex) {
  try {
    final cleaned = hex.replaceFirst('#', '');
    return Color(int.parse('FF$cleaned', radix: 16));
  } catch (_) {
    return const Color(0xFF71D5E3);
  }
}

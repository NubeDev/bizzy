import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../agents/application/agent_provider.dart';

class SessionsScreen extends ConsumerWidget {
  const SessionsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionsAsync = ref.watch(sessionsProvider);
    final colors = RubixTokens.of(context);

    return sessionsAsync.when(
      loading: () => const Center(child: RubixLoader()),
      error: (e, _) => Center(
        child: RubixErrorState(
          message: 'Failed to load sessions',
          detail: e.toString(),
          onRetry: () => ref.invalidate(sessionsProvider),
        ),
      ),
      data: (sessions) {
        if (sessions.isEmpty) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(LucideIcons.clock, size: 40, color: colors.textMuted),
                const SizedBox(height: 12),
                Text(
                  'No history yet',
                  style: colors.inter(
                    fontSize: 16,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Your conversations will appear here.',
                  style: colors.inter(
                    fontSize: 13,
                    color: colors.textMuted,
                  ),
                ),
              ],
            ),
          );
        }

        return Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: ListView.builder(
              padding: const EdgeInsets.symmetric(
                horizontal: 24,
                vertical: 20,
              ),
              itemCount: sessions.length,
              itemBuilder: (context, i) {
                final s = sessions[i];
                return _SessionRow(session: s, colors: colors);
              },
            ),
          ),
        );
      },
    );
  }
}

class _SessionRow extends StatefulWidget {
  final dynamic session;
  final RubixColors colors;

  const _SessionRow({required this.session, required this.colors});

  @override
  State<_SessionRow> createState() => _SessionRowState();
}

class _SessionRowState extends State<_SessionRow> {
  bool _hovered = false;

  @override
  Widget build(BuildContext context) {
    final s = widget.session;
    final colors = widget.colors;

    return MouseRegion(
      onEnter: (_) => setState(() => _hovered = true),
      onExit: (_) => setState(() => _hovered = false),
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: () => context.go('/sessions/${s.id}'),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 120),
          margin: const EdgeInsets.only(bottom: 2),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: _hovered ? colors.surfaceHover : Colors.transparent,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        if (s.agent.isNotEmpty) ...[
                          Text(
                            s.agent,
                            style: colors.inter(
                              fontSize: 14,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                          const SizedBox(width: 8),
                        ],
                        Text(
                          '${s.durationFormatted} · ${s.costFormatted}',
                          style: colors.mono(
                            fontSize: 11,
                            color: colors.textMuted,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      s.prompt,
                      style: colors.inter(
                        fontSize: 13,
                        color: colors.textSecondary,
                        height: 1.3,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Icon(LucideIcons.chevronRight,
                  size: 16, color: colors.textMuted),
            ],
          ),
        ),
      ),
    );
  }
}

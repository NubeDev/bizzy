import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../application/agent_provider.dart';
import '../models/agent.dart';

// ── Per-agent theming ──

class _AgentTheme {
  final IconData icon;
  final List<Color> gradient;

  const _AgentTheme({required this.icon, required this.gradient});
}

_AgentTheme _themeFor(String name) {
  final n = name.toLowerCase();
  if (n.contains('developer') || n.contains('rubix')) {
    return _AgentTheme(
      icon: LucideIcons.terminal,
      gradient: [const Color(0xFF71D5E3), const Color(0xFF309EAB)],
    );
  }
  if (n.contains('marketing') || n.contains('content')) {
    return _AgentTheme(
      icon: LucideIcons.megaphone,
      gradient: [const Color(0xFFE879F9), const Color(0xFFA855F7)],
    );
  }
  if (n.contains('admin') || n.contains('manage')) {
    return _AgentTheme(
      icon: LucideIcons.shield,
      gradient: [const Color(0xFFFBBF24), const Color(0xFFF59E0B)],
    );
  }
  if (n.contains('device') || n.contains('sensor') || n.contains('iot')) {
    return _AgentTheme(
      icon: LucideIcons.cpu,
      gradient: [const Color(0xFF34D399), const Color(0xFF10B981)],
    );
  }
  if (n.contains('data') || n.contains('analyt')) {
    return _AgentTheme(
      icon: LucideIcons.barChart2,
      gradient: [const Color(0xFF38BDF8), const Color(0xFF3B82F6)],
    );
  }
  // Default
  return _AgentTheme(
    icon: LucideIcons.bot,
    gradient: [const Color(0xFF71D5E3), const Color(0xFF309EAB)],
  );
}

class AgentsScreen extends ConsumerWidget {
  const AgentsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final agentsAsync = ref.watch(agentsProvider);
    final colors = RubixTokens.of(context);

    return agentsAsync.when(
      loading: () => const Center(child: RubixLoader()),
      error: (e, _) => Center(
        child: RubixErrorState(
          message: 'Failed to load agents',
          detail: e.toString(),
          onRetry: () => ref.invalidate(agentsProvider),
        ),
      ),
      data: (agents) {
        if (agents.isEmpty) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(LucideIcons.bot, size: 48, color: colors.textMuted),
                const SizedBox(height: 16),
                Text(
                  'No agents yet',
                  style: colors.inter(
                    fontSize: 18,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  'Install a skill on the server to get started.',
                  style: colors.inter(
                    fontSize: 14,
                    color: colors.textMuted,
                  ),
                ),
              ],
            ),
          );
        }

        return Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 700),
            child: ListView(
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 28),
              children: [
                // Page title
                Text(
                  'Skills',
                  style: colors.inter(
                    fontSize: 24,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  '${agents.length} available',
                  style: colors.inter(
                    fontSize: 14,
                    color: colors.textMuted,
                  ),
                ),
                const SizedBox(height: 24),

                // Agent cards
                for (int i = 0; i < agents.length; i++) ...[
                  if (i > 0) const SizedBox(height: 10),
                  _AgentCard(agent: agents[i], colors: colors),
                ],
              ],
            ),
          ),
        );
      },
    );
  }
}

class _AgentCard extends StatelessWidget {
  final Agent agent;
  final RubixColors colors;

  const _AgentCard({required this.agent, required this.colors});

  @override
  Widget build(BuildContext context) {
    final theme = _themeFor(agent.name);
    final qaCount = agent.qaTools.length;
    final toolCount = agent.tools.length;

    return Material(
      color: colors.surfaceBright,
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        borderRadius: BorderRadius.circular(14),
        onTap: () => context.go('/agent/${agent.name}'),
        child: Container(
          padding: const EdgeInsets.all(18),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: colors.border, width: 0.5),
          ),
          child: Row(
            children: [
              // Gradient icon
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    colors: theme.gradient,
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                  borderRadius: BorderRadius.circular(13),
                ),
                child: Icon(theme.icon, size: 24, color: Colors.white),
              ),
              const SizedBox(width: 16),

              // Name + description
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      agent.name,
                      style: colors.inter(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    if (agent.description.isNotEmpty) ...[
                      const SizedBox(height: 3),
                      Text(
                        agent.description,
                        style: colors.inter(
                          fontSize: 13,
                          color: colors.textSecondary,
                          height: 1.4,
                        ),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                    if (toolCount > 0) ...[
                      const SizedBox(height: 10),
                      Row(
                        children: [
                          _Tag(
                            icon: LucideIcons.wrench,
                            label: '$toolCount tools',
                            colors: colors,
                          ),
                          if (qaCount > 0) ...[
                            const SizedBox(width: 8),
                            _Tag(
                              icon: LucideIcons.messageSquare,
                              label: '$qaCount flows',
                              color: theme.gradient[0],
                              colors: colors,
                            ),
                          ],
                        ],
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 12),
              Icon(LucideIcons.chevronRight, size: 16, color: colors.textMuted),
            ],
          ),
        ),
      ),
    );
  }
}

class _Tag extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color? color;
  final RubixColors colors;

  const _Tag({
    required this.icon,
    required this.label,
    this.color,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    final c = color ?? colors.textMuted;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: c.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 12, color: c),
          const SizedBox(width: 4),
          Text(
            label,
            style: colors.mono(fontSize: 11, color: c),
          ),
        ],
      ),
    );
  }
}

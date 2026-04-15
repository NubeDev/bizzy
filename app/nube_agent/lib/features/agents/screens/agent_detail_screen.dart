import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../application/agent_provider.dart';
import '../models/agent.dart';

class AgentDetailScreen extends ConsumerWidget {
  final String agentName;

  const AgentDetailScreen({super.key, required this.agentName});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final agentsAsync = ref.watch(agentsProvider);
    final colors = RubixTokens.of(context);

    return Column(
      children: [
        // Header
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            border: Border(
              bottom: BorderSide(color: colors.border, width: 0.5),
            ),
          ),
          child: Row(
            children: [
              InkWell(
                borderRadius: BorderRadius.circular(6),
                onTap: () => context.go('/agents'),
                child: Padding(
                  padding: const EdgeInsets.all(4),
                  child: Icon(LucideIcons.arrowLeft,
                      size: 18, color: colors.textSecondary),
                ),
              ),
              const SizedBox(width: 12),
              Text(
                agentName,
                style: colors.inter(fontSize: 15, fontWeight: FontWeight.w500),
              ),
            ],
          ),
        ),

        // Content
        Expanded(
          child: agentsAsync.when(
            loading: () => const Center(child: RubixLoader()),
            error: (e, _) => Center(
              child: RubixErrorState(
                message: 'Failed to load agent',
                detail: e.toString(),
                onRetry: () => ref.invalidate(agentsProvider),
              ),
            ),
            data: (agents) {
              final agent = agents
                  .cast<Agent?>()
                  .firstWhere((a) => a!.name == agentName, orElse: () => null);
              if (agent == null) {
                return const Center(
                  child: RubixEmptyState(
                    title: 'Agent not found',
                    icon: LucideIcons.alertCircle,
                  ),
                );
              }
              return _AgentContent(agent: agent, colors: colors);
            },
          ),
        ),
      ],
    );
  }
}

class _AgentContent extends StatelessWidget {
  final Agent agent;
  final RubixColors colors;

  const _AgentContent({required this.agent, required this.colors});

  /// Pick a nice icon for QA flows based on the tool name.
  static IconData _qaIcon(String toolName) {
    final name = toolName.toLowerCase();
    if (name.contains('marketing') || name.contains('plan')) {
      return LucideIcons.target;
    }
    if (name.contains('review') || name.contains('content')) {
      return LucideIcons.fileSearch;
    }
    if (name.contains('create') || name.contains('build') || name.contains('skill')) {
      return LucideIcons.sparkles;
    }
    if (name.contains('deploy') || name.contains('launch')) {
      return LucideIcons.rocket;
    }
    if (name.contains('analyze') || name.contains('report')) {
      return LucideIcons.barChart2;
    }
    if (name.contains('device') || name.contains('sensor')) {
      return LucideIcons.cpu;
    }
    return LucideIcons.clipboardList;
  }

  /// Make a nice human-readable title from tool name.
  static String _qaTitle(AgentTool tool) {
    return tool.shortName
        .split(' ')
        .map((w) => w.isNotEmpty ? '${w[0].toUpperCase()}${w.substring(1)}' : '')
        .join(' ');
  }

  /// Make a short action-oriented subtitle.
  static String _qaSubtitle(AgentTool tool) {
    final desc = tool.description;
    if (desc.isEmpty) return 'Answer a few questions to get started';
    // Truncate long descriptions and make them action-oriented.
    if (desc.length > 80) return '${desc.substring(0, 77)}...';
    return desc;
  }

  @override
  Widget build(BuildContext context) {
    final qaTools = agent.qaTools;
    final nonQaTools = agent.tools.where((t) => !t.isQa).toList();

    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 580),
        child: ListView(
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
          children: [
            // ── Agent hero ──
            Center(
              child: Column(
                children: [
                  // Gradient icon
                  Container(
                    width: 64,
                    height: 64,
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        colors: [
                          RubixTokens.accentCool,
                          RubixTokens.accentCoolDeep,
                        ],
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                      borderRadius: BorderRadius.circular(18),
                      boxShadow: RubixTokens.glowMint(opacity: 0.2, blur: 24),
                    ),
                    child: const Icon(
                      LucideIcons.bot,
                      size: 30,
                      color: Colors.black,
                    ),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    agent.name,
                    style: colors.inter(
                      fontSize: 22,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (agent.description.isNotEmpty) ...[
                    const SizedBox(height: 6),
                    Text(
                      agent.description,
                      style: colors.inter(
                        fontSize: 14,
                        color: colors.textSecondary,
                        height: 1.4,
                      ),
                      textAlign: TextAlign.center,
                    ),
                  ],
                ],
              ),
            ),

            const SizedBox(height: 32),

            // ── Section: What would you like to do? ──
            _SectionLabel(text: 'What would you like to do?', colors: colors),
            const SizedBox(height: 12),

            // Chat — always first
            _ActionCard(
              icon: LucideIcons.messageSquare,
              iconGradient: const [Color(0xFF71D5E3), Color(0xFF309EAB)],
              title: 'Chat',
              subtitle: 'Ask anything — open conversation',
              onTap: () => context.go('/chat/${agent.name}'),
              colors: colors,
            ),

            // QA flows as action cards
            for (final tool in qaTools) ...[
              const SizedBox(height: 8),
              _ActionCard(
                icon: _qaIcon(tool.name),
                iconGradient: _gradientForTool(tool.name),
                title: _qaTitle(tool),
                subtitle: _qaSubtitle(tool),
                onTap: () =>
                    context.go('/qa/${Uri.encodeComponent(tool.name)}'),
                colors: colors,
              ),
            ],

            // ── Capabilities ──
            if (nonQaTools.isNotEmpty) ...[
              const SizedBox(height: 32),
              _SectionLabel(text: 'Capabilities', colors: colors),
              const SizedBox(height: 10),
              Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: colors.surfaceBright,
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(color: colors.border, width: 0.5),
                ),
                child: Column(
                  children: [
                    for (int i = 0; i < nonQaTools.length; i++) ...[
                      if (i > 0)
                        Divider(
                          height: 1,
                          color: colors.border.withValues(alpha: 0.5),
                        ),
                      _CapabilityRow(tool: nonQaTools[i], colors: colors),
                    ],
                  ],
                ),
              ),
            ],

            const SizedBox(height: 32),
          ],
        ),
      ),
    );
  }

  static List<Color> _gradientForTool(String name) {
    final n = name.toLowerCase();
    if (n.contains('marketing') || n.contains('plan')) {
      return [const Color(0xFFE879F9), const Color(0xFFA855F7)]; // purple
    }
    if (n.contains('review') || n.contains('content')) {
      return [const Color(0xFF38BDF8), const Color(0xFF3B82F6)]; // blue
    }
    if (n.contains('create') || n.contains('build') || n.contains('skill')) {
      return [const Color(0xFFFBBF24), const Color(0xFFF59E0B)]; // amber
    }
    return [const Color(0xFF34D399), const Color(0xFF10B981)]; // green
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  final RubixColors colors;

  const _SectionLabel({required this.text, required this.colors});

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: colors.inter(
        fontSize: 12,
        fontWeight: FontWeight.w600,
        color: colors.textMuted,
        letterSpacing: 0.3,
      ),
    );
  }
}

class _ActionCard extends StatelessWidget {
  final IconData icon;
  final List<Color> iconGradient;
  final String title;
  final String subtitle;
  final VoidCallback onTap;
  final RubixColors colors;

  const _ActionCard({
    required this.icon,
    required this.iconGradient,
    required this.title,
    required this.subtitle,
    required this.onTap,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
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
          child: Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    colors: iconGradient,
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                  borderRadius: BorderRadius.circular(11),
                ),
                child: Icon(icon, size: 22, color: Colors.white),
              ),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: colors.inter(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      subtitle,
                      style: colors.inter(
                        fontSize: 13,
                        color: colors.textMuted,
                        height: 1.3,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Icon(LucideIcons.chevronRight, size: 16, color: colors.textMuted),
            ],
          ),
        ),
      ),
    );
  }
}

/// Compact non-clickable tool row for the capabilities section.
class _CapabilityRow extends StatelessWidget {
  final AgentTool tool;
  final RubixColors colors;

  const _CapabilityRow({required this.tool, required this.colors});

  IconData get _icon {
    if (tool.type == 'openapi') return LucideIcons.globe;
    final name = tool.name.toLowerCase();
    if (name.contains('device')) return LucideIcons.cpu;
    if (name.contains('restart')) return LucideIcons.refreshCw;
    if (name.contains('search') || name.contains('summary')) {
      return LucideIcons.search;
    }
    return LucideIcons.wrench;
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Icon(_icon, size: 15, color: colors.textMuted),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  tool.shortName,
                  style: colors.inter(fontSize: 13, fontWeight: FontWeight.w500),
                ),
                if (tool.description.isNotEmpty)
                  Text(
                    tool.description,
                    style: colors.inter(
                      fontSize: 12,
                      color: colors.textMuted,
                      height: 1.3,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

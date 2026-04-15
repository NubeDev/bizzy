import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../../core/theme/markdown_theme.dart';
import '../../agents/application/agent_provider.dart';

class SessionDetailScreen extends ConsumerWidget {
  final String sessionId;

  const SessionDetailScreen({super.key, required this.sessionId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionAsync = ref.watch(sessionDetailProvider(sessionId));
    final colors = RubixTokens.of(context);

    return Column(
      children: [
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
                onTap: () => context.go('/sessions'),
                child: Padding(
                  padding: const EdgeInsets.all(4),
                  child: Icon(LucideIcons.arrowLeft,
                      size: 18, color: colors.textSecondary),
                ),
              ),
              const SizedBox(width: 12),
              Text(
                sessionId,
                style: colors.mono(fontSize: 13, color: colors.textSecondary),
              ),
            ],
          ),
        ),
        Expanded(
          child: sessionAsync.when(
            loading: () => const Center(child: RubixLoader()),
            error: (e, _) => Center(
              child: RubixErrorState(
                message: 'Failed to load session',
                detail: e.toString(),
                onRetry: () => ref.invalidate(sessionDetailProvider(sessionId)),
              ),
            ),
            data: (session) => SingleChildScrollView(
              padding: const EdgeInsets.symmetric(
                horizontal: Spacing.lg,
                vertical: Spacing.md,
              ),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 820),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Meta row
                      RubixCard(
                        inset: true,
                        child: Row(
                          children: [
                            _meta(colors, 'Agent', session.agent),
                            _meta(colors, 'Duration', session.durationFormatted),
                            _meta(colors, 'Cost', session.costFormatted),
                          ],
                        ),
                      ),
                      const SizedBox(height: Spacing.lg),

                      // Prompt
                      Text('PROMPT',
                          style: colors.mono(
                              fontSize: 11, color: colors.textMuted)),
                      const SizedBox(height: 6),
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(Spacing.md),
                        decoration: BoxDecoration(
                          color: RubixTokens.accentCool.withValues(alpha: 0.08),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Text(
                          session.prompt,
                          style: colors.inter(fontSize: 14.5, height: 1.5),
                        ),
                      ),
                      const SizedBox(height: Spacing.lg),

                      // Result
                      Text('RESULT',
                          style: colors.mono(
                              fontSize: 11, color: colors.textMuted)),
                      const SizedBox(height: 6),
                      MarkdownBody(
                        data: session.result,
                        selectable: true,
                        styleSheet: buildMarkdownTheme(context),
                      ),
                      const SizedBox(height: Spacing.xl),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _meta(RubixColors colors, String label, String value) {
    return Expanded(
      child: Column(
        children: [
          Text(label,
              style: colors.mono(fontSize: 10, color: colors.textMuted)),
          const SizedBox(height: 2),
          Text(value,
              style:
                  colors.mono(fontSize: 13, color: RubixTokens.accentCool)),
        ],
      ),
    );
  }
}

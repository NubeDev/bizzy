import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../../core/theme/markdown_theme.dart';
import '../application/qa_provider.dart';

/// Renders the streamed AI result — full-width, no card wrapping.
class QaResultView extends StatelessWidget {
  final QaSessionState qaState;
  final VoidCallback? onRestart;

  const QaResultView({
    super.key,
    required this.qaState,
    this.onRestart,
  });

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Tool calls
        if (qaState.toolCalls.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: Spacing.sm),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (final tool in qaState.toolCalls)
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: 3),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(LucideIcons.settings,
                            size: 13, color: colors.textMuted),
                        const SizedBox(width: 6),
                        Text(
                          'calling $tool',
                          style:
                              colors.mono(fontSize: 12, color: colors.textMuted),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),

        // Markdown result — full-width, no card
        if (qaState.resultText.isNotEmpty)
          MarkdownBody(
            data: qaState.resultText,
            selectable: true,
            styleSheet: buildMarkdownTheme(context),
          ),

        // Done summary bar
        if (qaState.phase == QaPhase.done) ...[
          const SizedBox(height: Spacing.lg),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(
              horizontal: Spacing.md,
              vertical: Spacing.sm,
            ),
            decoration: BoxDecoration(
              color: colors.surfaceWell,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(LucideIcons.checkCircle,
                    size: 14, color: RubixTokens.statusSuccess),
                const SizedBox(width: Spacing.xs),
                Text(
                  '${qaState.sessionId ?? ""} · ${qaState.durationFormatted} · ${qaState.costFormatted}',
                  style:
                      colors.mono(fontSize: 12, color: colors.textSecondary),
                ),
                const Spacer(),
                // Copy
                _ActionIcon(
                  icon: LucideIcons.copy,
                  tooltip: 'Copy',
                  onTap: () {
                    Clipboard.setData(
                        ClipboardData(text: qaState.resultText));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(
                        content: Text('Copied to clipboard'),
                        duration: Duration(seconds: 1),
                      ),
                    );
                  },
                  colors: colors,
                ),
                if (onRestart != null) ...[
                  const SizedBox(width: Spacing.xs),
                  _ActionIcon(
                    icon: LucideIcons.refreshCw,
                    tooltip: 'Start new',
                    onTap: onRestart!,
                    colors: colors,
                  ),
                ],
              ],
            ),
          ),
        ],
      ],
    );
  }
}

class _ActionIcon extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;
  final RubixColors colors;

  const _ActionIcon({
    required this.icon,
    required this.tooltip,
    required this.onTap,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: InkWell(
        borderRadius: BorderRadius.circular(4),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(4),
          child: Icon(icon, size: 15, color: colors.textMuted),
        ),
      ),
    );
  }
}

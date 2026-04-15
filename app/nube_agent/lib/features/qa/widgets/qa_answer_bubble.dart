import 'package:flutter/material.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../models/qa_field.dart';

/// Collapsed view of a completed question + answer pair.
class QaAnswerBubble extends StatelessWidget {
  final QaExchange exchange;

  const QaAnswerBubble({super.key, required this.exchange});

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: RubixCard(
        inset: true,
        padding: const EdgeInsets.symmetric(
          horizontal: Spacing.sm,
          vertical: Spacing.xs,
        ),
        child: Row(
          children: [
            Icon(LucideIcons.check, size: 14, color: RubixTokens.statusSuccess),
            const SizedBox(width: Spacing.xs),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    exchange.question.label,
                    style: colors.inter(
                      fontSize: 12,
                      color: colors.textMuted,
                    ),
                  ),
                  Text(
                    exchange.displayAnswer,
                    style: colors.inter(
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

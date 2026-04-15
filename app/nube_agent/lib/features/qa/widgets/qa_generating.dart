import 'package:flutter/material.dart';
import 'package:rubix_ui/rubix_ui.dart';

/// "Generating..." indicator shown between questions and AI response.
class QaGenerating extends StatelessWidget {
  final String message;

  const QaGenerating({super.key, required this.message});

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);

    return RubixCard(
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const RubixLoader(size: 18),
          const SizedBox(width: Spacing.sm),
          Expanded(
            child: Text(
              message,
              style: colors.inter(fontSize: 14, color: colors.textSecondary),
            ),
          ),
        ],
      ),
    );
  }
}

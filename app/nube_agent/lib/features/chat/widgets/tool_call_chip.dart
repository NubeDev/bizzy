import 'package:flutter/material.dart';
import 'package:rubix_ui/rubix_ui.dart';

class ToolCallChip extends StatelessWidget {
  final String toolName;

  const ToolCallChip({super.key, required this.toolName});

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: colors.surfaceBright,
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: colors.border, width: 0.5),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(
                strokeWidth: 1.5,
                color: RubixTokens.accentCool,
              ),
            ),
            const SizedBox(width: 8),
            Text(
              toolName,
              style: colors.mono(fontSize: 12, color: colors.textSecondary),
            ),
          ],
        ),
      ),
    );
  }
}

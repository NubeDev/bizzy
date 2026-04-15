import 'package:flutter/material.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../models/qa_field.dart';
import 'qa_multi_select.dart';
import 'qa_number_input.dart';
import 'qa_select_input.dart';
import 'qa_text_input.dart';

/// Renders the current active question with the appropriate input widget.
class QaQuestionCard extends StatelessWidget {
  final QaField field;
  final ValueChanged<dynamic> onAnswer;

  const QaQuestionCard({
    super.key,
    required this.field,
    required this.onAnswer,
  });

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);

    return RubixCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            field.label,
            style: colors.inter(fontSize: 15, fontWeight: FontWeight.w600),
          ),
          if (field.required)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Text(
                'Required',
                style: colors.mono(fontSize: 10, color: colors.textMuted),
              ),
            ),
          const SizedBox(height: Spacing.sm),
          _buildInput(),
        ],
      ),
    );
  }

  Widget _buildInput() {
    return switch (field.input) {
      'text' || 'textarea' => QaTextInput(
          field: field,
          onSubmit: onAnswer,
        ),
      'select' => QaSelectInput(
          field: field,
          onSubmit: onAnswer,
        ),
      'multi_select' => QaMultiSelect(
          field: field,
          onSubmit: onAnswer,
        ),
      'number' => QaNumberInput(
          field: field,
          onSubmit: onAnswer,
        ),
      _ => QaTextInput(
          field: field,
          onSubmit: onAnswer,
        ),
    };
  }
}

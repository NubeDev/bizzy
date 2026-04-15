import 'package:flutter/material.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../models/qa_field.dart';

/// Multi-select chip group for QA questions.
class QaMultiSelect extends StatefulWidget {
  final QaField field;
  final ValueChanged<String> onSubmit;

  const QaMultiSelect({super.key, required this.field, required this.onSubmit});

  @override
  State<QaMultiSelect> createState() => _QaMultiSelectState();
}

class _QaMultiSelectState extends State<QaMultiSelect> {
  final Set<String> _selected = {};

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Wrap(
          spacing: Spacing.xs,
          runSpacing: Spacing.xs,
          children: widget.field.options.map((opt) {
            final isOn = _selected.contains(opt.value);
            return FilterChip(
              label: Text(
                opt.label,
                style: colors.inter(
                  fontSize: 13,
                  color: isOn ? RubixTokens.accentCool : colors.textSecondary,
                ),
              ),
              selected: isOn,
              onSelected: (on) {
                setState(() {
                  if (on) {
                    _selected.add(opt.value);
                  } else {
                    _selected.remove(opt.value);
                  }
                });
              },
              selectedColor: RubixTokens.accentCool.withValues(alpha: 0.15),
              backgroundColor: colors.surfaceWell,
              checkmarkColor: RubixTokens.accentCool,
              side: BorderSide.none,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(RubixTokens.radiusPill),
              ),
            );
          }).toList(),
        ),
        const SizedBox(height: Spacing.sm),
        RubixButton.primary(
          onPressed: _selected.isNotEmpty
              ? () => widget.onSubmit(_selected.join(','))
              : null,
          label: 'Continue',
        ),
      ],
    );
  }
}

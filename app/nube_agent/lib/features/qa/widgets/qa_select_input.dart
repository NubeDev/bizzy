import 'package:flutter/material.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../models/qa_field.dart';

/// Single-select radio list for QA questions.
class QaSelectInput extends StatefulWidget {
  final QaField field;
  final ValueChanged<String> onSubmit;

  const QaSelectInput({super.key, required this.field, required this.onSubmit});

  @override
  State<QaSelectInput> createState() => _QaSelectInputState();
}

class _QaSelectInputState extends State<QaSelectInput> {
  String? _selected;

  @override
  void initState() {
    super.initState();
    _selected = widget.field.defaultValue?.toString();
  }

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ...widget.field.options.map((opt) => Padding(
              padding: const EdgeInsets.symmetric(vertical: 2),
              child: Material(
                color: _selected == opt.value
                    ? RubixTokens.accentCool.withValues(alpha: 0.1)
                    : Colors.transparent,
                borderRadius: BorderRadius.circular(RubixTokens.radiusPill),
                child: InkWell(
                  borderRadius: BorderRadius.circular(RubixTokens.radiusPill),
                  onTap: () => setState(() => _selected = opt.value),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: Spacing.sm,
                      vertical: Spacing.xs,
                    ),
                    child: Row(
                      children: [
                        Icon(
                          _selected == opt.value
                              ? Icons.radio_button_checked
                              : Icons.radio_button_unchecked,
                          size: 18,
                          color: _selected == opt.value
                              ? RubixTokens.accentCool
                              : colors.textMuted,
                        ),
                        const SizedBox(width: Spacing.sm),
                        Expanded(
                          child: Text(
                            opt.label,
                            style: colors.inter(fontSize: 14),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            )),
        const SizedBox(height: Spacing.sm),
        RubixButton.primary(
          onPressed: _selected != null ? () => widget.onSubmit(_selected!) : null,
          label: 'Continue',
        ),
      ],
    );
  }
}

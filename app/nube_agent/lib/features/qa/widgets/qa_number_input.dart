import 'package:flutter/material.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../models/qa_field.dart';

/// Number input for QA questions.
class QaNumberInput extends StatefulWidget {
  final QaField field;
  final ValueChanged<num> onSubmit;

  const QaNumberInput({super.key, required this.field, required this.onSubmit});

  @override
  State<QaNumberInput> createState() => _QaNumberInputState();
}

class _QaNumberInputState extends State<QaNumberInput> {
  late final TextEditingController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = TextEditingController(
      text: widget.field.defaultValue?.toString() ?? '',
    );
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  void _submit() {
    final text = _ctrl.text.trim();
    if (text.isEmpty && widget.field.required) return;
    final value = num.tryParse(text);
    if (value == null) return;
    widget.onSubmit(value);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        RubixInput(
          controller: _ctrl,
          placeholder: widget.field.placeholder.isNotEmpty
              ? widget.field.placeholder
              : 'Enter a number',
          keyboardType: TextInputType.number,
        ),
        const SizedBox(height: Spacing.sm),
        RubixButton.primary(
          onPressed: _submit,
          label: 'Continue',
        ),
      ],
    );
  }
}

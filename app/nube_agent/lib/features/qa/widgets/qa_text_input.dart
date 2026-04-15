import 'package:flutter/material.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../models/qa_field.dart';

/// Text/textarea input for QA questions.
class QaTextInput extends StatefulWidget {
  final QaField field;
  final ValueChanged<String> onSubmit;

  const QaTextInput({super.key, required this.field, required this.onSubmit});

  @override
  State<QaTextInput> createState() => _QaTextInputState();
}

class _QaTextInputState extends State<QaTextInput> {
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

  String? _validate() {
    final text = _ctrl.text.trim();
    if (widget.field.required && text.isEmpty) return 'Required';
    if (widget.field.minLength > 0 && text.length < widget.field.minLength) {
      return 'Min ${widget.field.minLength} characters';
    }
    if (widget.field.maxLength > 0 && text.length > widget.field.maxLength) {
      return 'Max ${widget.field.maxLength} characters';
    }
    return null;
  }

  void _submit() {
    final error = _validate();
    if (error != null) return;
    widget.onSubmit(_ctrl.text.trim());
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
              : null,
          maxLines: widget.field.input == 'textarea' ? 5 : 1,
          validator: (_) => _validate(),
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

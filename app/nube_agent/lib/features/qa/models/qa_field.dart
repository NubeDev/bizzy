/// A question field received from the server during a QA flow.
class QaField {
  final String field;
  final String label;
  final String input; // "text", "textarea", "select", "multi_select", "number"
  final bool required;
  final dynamic defaultValue;
  final String placeholder;
  final List<QaOption> options;
  final int minLength;
  final int maxLength;

  const QaField({
    required this.field,
    required this.label,
    this.input = 'text',
    this.required = false,
    this.defaultValue,
    this.placeholder = '',
    this.options = const [],
    this.minLength = 0,
    this.maxLength = 0,
  });

  factory QaField.fromJson(Map<String, dynamic> json) {
    return QaField(
      field: json['field'] as String? ?? '',
      label: json['label'] as String? ?? '',
      input: json['input'] as String? ?? 'text',
      required: json['required'] as bool? ?? false,
      defaultValue: json['default'],
      placeholder: json['placeholder'] as String? ?? '',
      options: (json['options'] as List<dynamic>?)
              ?.map((o) => QaOption.fromJson(o as Map<String, dynamic>))
              .toList() ??
          [],
      minLength: json['min_length'] as int? ?? 0,
      maxLength: json['max_length'] as int? ?? 0,
    );
  }
}

class QaOption {
  final String value;
  final String label;

  const QaOption({required this.value, required this.label});

  factory QaOption.fromJson(Map<String, dynamic> json) {
    return QaOption(
      value: json['value'] as String? ?? '',
      label: json['label'] as String? ?? json['value'] as String? ?? '',
    );
  }
}

/// A completed question-answer exchange.
class QaExchange {
  final QaField question;
  final dynamic answer;

  const QaExchange({required this.question, required this.answer});

  String get displayAnswer {
    if (answer is List) return (answer as List).join(', ');
    return answer.toString();
  }
}

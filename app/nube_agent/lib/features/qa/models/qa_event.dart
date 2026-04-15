import 'qa_field.dart';

enum QaEventType {
  session,
  question,
  generating,
  connected,
  toolCall,
  text,
  done,
  error,
}

class QaEvent {
  final QaEventType type;
  final String? sessionId;

  // Question fields
  final QaField? field;

  // Generating
  final String? message;

  // Connected
  final String? model;

  // Tool call
  final String? toolName;

  // Text
  final String? content;

  // Error
  final String? error;

  // Done
  final int? durationMs;
  final double? costUsd;

  // Skill generator output (present when type=done and tool returns structured data)
  final Map<String, dynamic>? createApp;
  final Map<String, dynamic>? createPrompt;
  final Map<String, dynamic>? createTool;

  const QaEvent({
    required this.type,
    this.sessionId,
    this.field,
    this.message,
    this.model,
    this.toolName,
    this.content,
    this.error,
    this.durationMs,
    this.costUsd,
    this.createApp,
    this.createPrompt,
    this.createTool,
  });

  factory QaEvent.fromJson(Map<String, dynamic> json) {
    final typeStr = json['type'] as String? ?? '';
    final type = switch (typeStr) {
      'session' => QaEventType.session,
      'question' => QaEventType.question,
      'generating' => QaEventType.generating,
      'connected' => QaEventType.connected,
      'tool_call' => QaEventType.toolCall,
      'text' => QaEventType.text,
      'done' => QaEventType.done,
      'error' => QaEventType.error,
      _ => QaEventType.error,
    };

    return QaEvent(
      type: type,
      sessionId: json['session_id'] as String?,
      field: type == QaEventType.question ? QaField.fromJson(json) : null,
      message: json['message'] as String?,
      model: json['model'] as String?,
      toolName: json['name'] as String?,
      content: json['content'] as String?,
      error: json['error'] as String?,
      durationMs: json['duration_ms'] as int?,
      costUsd: (json['cost_usd'] as num?)?.toDouble(),
      createApp: json['create_app'] as Map<String, dynamic>?,
      createPrompt: json['create_prompt'] as Map<String, dynamic>?,
      createTool: json['create_tool'] as Map<String, dynamic>?,
    );
  }
}

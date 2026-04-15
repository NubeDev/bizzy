enum ChatMessageType { userMessage, text, toolCall, connected, done, error }

class ChatMessage {
  final ChatMessageType type;
  final String content;
  final String? toolName;
  final String? model;
  final String? sessionId;
  final int? durationMs;
  final double? costUsd;
  final DateTime timestamp;

  const ChatMessage({
    required this.type,
    this.content = '',
    this.toolName,
    this.model,
    this.sessionId,
    this.durationMs,
    this.costUsd,
    required this.timestamp,
  });

  factory ChatMessage.user(String text) => ChatMessage(
        type: ChatMessageType.userMessage,
        content: text,
        timestamp: DateTime.now(),
      );

  String get durationFormatted {
    if (durationMs == null) return '';
    return '${(durationMs! / 1000).toStringAsFixed(1)}s';
  }

  String get costFormatted {
    if (costUsd == null) return '';
    return '\$${costUsd!.toStringAsFixed(3)}';
  }
}

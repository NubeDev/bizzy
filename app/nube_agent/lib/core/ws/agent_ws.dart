import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';
import '../../features/chat/models/chat_message.dart';

/// Streams agent events over WebSocket.
///
/// Protocol:
///   1. Connect to ws(s)://server/api/agents/run?token=xxx
///   2. Receive {"type":"session","session_id":"ses-..."}
///   3. Send   {"prompt":"...","agent":"..."}
///   4. Stream events until "done" or "error"
class AgentWS {
  final String serverUrl;
  final String token;

  WebSocketChannel? _channel;
  String? sessionId;

  AgentWS({required this.serverUrl, required this.token});

  /// Opens a WS connection, sends the prompt after receiving the session event,
  /// and yields [ChatMessage]s for each server event.
  Stream<ChatMessage> run(String prompt, {String? agent}) async* {
    final wsScheme = serverUrl.startsWith('https') ? 'wss' : 'ws';
    final host = serverUrl.replaceFirst(RegExp(r'^https?://'), '');
    final uri = Uri.parse('$wsScheme://$host/api/agents/run?token=$token');
    _channel = WebSocketChannel.connect(uri);

    await _channel!.ready;

    bool promptSent = false;

    await for (final raw in _channel!.stream) {
      final data = jsonDecode(raw as String) as Map<String, dynamic>;
      final type = data['type'] as String? ?? '';

      switch (type) {
        case 'session':
          sessionId = data['session_id'] as String?;
          // Send prompt after receiving session.
          if (!promptSent) {
            promptSent = true;
            _channel!.sink.add(jsonEncode({
              'prompt': prompt,
              if (agent case final a?) 'agent': a,
            }));
          }

        case 'connected':
          yield ChatMessage(
            type: ChatMessageType.connected,
            model: data['model'] as String?,
            sessionId: sessionId,
            timestamp: DateTime.now(),
          );

        case 'text':
          yield ChatMessage(
            type: ChatMessageType.text,
            content: data['content'] as String? ?? '',
            sessionId: sessionId,
            timestamp: DateTime.now(),
          );

        case 'tool_call':
          yield ChatMessage(
            type: ChatMessageType.toolCall,
            toolName: data['name'] as String?,
            sessionId: sessionId,
            timestamp: DateTime.now(),
          );

        case 'error':
          yield ChatMessage(
            type: ChatMessageType.error,
            content: data['error'] as String? ?? 'Unknown error',
            sessionId: sessionId,
            timestamp: DateTime.now(),
          );

        case 'done':
          yield ChatMessage(
            type: ChatMessageType.done,
            sessionId: sessionId,
            durationMs: data['duration_ms'] as int?,
            costUsd: (data['cost_usd'] as num?)?.toDouble(),
            timestamp: DateTime.now(),
          );
      }

      if (type == 'done' || type == 'error') break;
    }
  }

  void close() {
    _channel?.sink.close();
    _channel = null;
  }
}

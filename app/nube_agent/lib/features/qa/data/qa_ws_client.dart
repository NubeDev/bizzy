import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/qa_event.dart';

/// WebSocket client for the QA flow at /api/agents/qa.
///
/// Unlike the agent run WS, this is interactive — the client sends answers
/// between server questions, so we expose the channel for bidirectional use.
class QaWsClient {
  final String serverUrl;
  final String token;

  WebSocketChannel? _channel;

  QaWsClient({required this.serverUrl, required this.token});

  /// Connects and yields events. Auto-sends the flow name after the session event.
  Stream<QaEvent> connect(String flow) async* {
    final wsScheme = serverUrl.startsWith('https') ? 'wss' : 'ws';
    final host = serverUrl.replaceFirst(RegExp(r'^https?://'), '');
    final uri = Uri.parse('$wsScheme://$host/api/agents/qa?token=$token');
    _channel = WebSocketChannel.connect(uri);

    await _channel!.ready;

    await for (final raw in _channel!.stream) {
      final data = jsonDecode(raw as String) as Map<String, dynamic>;
      final event = QaEvent.fromJson(data);

      yield event;

      // Auto-send flow name after session event.
      if (event.type == QaEventType.session) {
        _channel!.sink.add(jsonEncode({'flow': flow}));
      }

      if (event.type == QaEventType.done ||
          event.type == QaEventType.error) {
        break;
      }
    }
  }

  /// Sends an answer to the current question.
  void answer(dynamic value) {
    _channel?.sink.add(jsonEncode({'answer': value}));
  }

  void close() {
    _channel?.sink.close();
    _channel = null;
  }
}

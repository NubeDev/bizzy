import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/auth/auth_repository.dart';
import '../../../core/ws/agent_ws.dart';
import '../../agents/application/auth_provider.dart';
import '../models/chat_message.dart';

/// Each chat screen gets its own ChatNotifier keyed by agent name.
final chatProvider =
    StateNotifierProvider.family<ChatNotifier, ChatState, String>(
  (ref, agentName) {
    final auth = ref.watch(authProvider).valueOrNull;
    return ChatNotifier(agentName: agentName, credentials: auth);
  },
);

class ChatState {
  final List<ChatMessage> messages;
  final bool isRunning;
  final String? error;

  const ChatState({
    this.messages = const [],
    this.isRunning = false,
    this.error,
  });

  ChatState copyWith({
    List<ChatMessage>? messages,
    bool? isRunning,
    String? error,
  }) {
    return ChatState(
      messages: messages ?? this.messages,
      isRunning: isRunning ?? this.isRunning,
      error: error,
    );
  }
}

class ChatNotifier extends StateNotifier<ChatState> {
  final String agentName;
  final AuthCredentials? credentials;
  AgentWS? _ws;

  /// Accumulates streamed text chunks into a single message.
  StringBuffer _textBuffer = StringBuffer();

  ChatNotifier({required this.agentName, this.credentials})
      : super(const ChatState());

  Future<void> send(String prompt) async {
    if (credentials == null || prompt.trim().isEmpty) return;

    // Add user message.
    final userMsg = ChatMessage.user(prompt);
    state = state.copyWith(
      messages: [...state.messages, userMsg],
      isRunning: true,
      error: null,
    );

    _textBuffer = StringBuffer();
    _ws = AgentWS(serverUrl: credentials!.serverUrl, token: credentials!.token);

    try {
      await for (final event in _ws!.run(prompt, agent: agentName)) {
        if (!mounted) break;

        switch (event.type) {
          case ChatMessageType.text:
            // Accumulate text into a single growing message.
            _textBuffer.write(event.content);
            final textMsg = ChatMessage(
              type: ChatMessageType.text,
              content: _textBuffer.toString(),
              sessionId: event.sessionId,
              timestamp: event.timestamp,
            );
            // Replace the last text message or append.
            final msgs = List<ChatMessage>.from(state.messages);
            if (msgs.isNotEmpty && msgs.last.type == ChatMessageType.text) {
              msgs[msgs.length - 1] = textMsg;
            } else {
              msgs.add(textMsg);
            }
            state = state.copyWith(messages: msgs);

          case ChatMessageType.toolCall:
          case ChatMessageType.connected:
            state = state.copyWith(
              messages: [...state.messages, event],
            );

          case ChatMessageType.done:
            state = state.copyWith(
              messages: [...state.messages, event],
              isRunning: false,
            );

          case ChatMessageType.error:
            state = state.copyWith(
              messages: [...state.messages, event],
              isRunning: false,
              error: event.content,
            );

          case ChatMessageType.userMessage:
            break; // Won't come from WS.
        }
      }
    } catch (e) {
      if (mounted) {
        state = state.copyWith(isRunning: false, error: e.toString());
      }
    }

    // Reset for next run.
    if (mounted && state.isRunning) {
      state = state.copyWith(isRunning: false);
    }
  }

  void clearMessages() {
    state = const ChatState();
  }

  @override
  void dispose() {
    _ws?.close();
    super.dispose();
  }
}

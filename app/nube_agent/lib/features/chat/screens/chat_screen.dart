import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../application/chat_provider.dart';
import '../models/chat_message.dart';
import '../widgets/chat_input.dart';
import '../widgets/message_bubble.dart';
import '../widgets/tool_call_chip.dart';

class ChatScreen extends ConsumerStatefulWidget {
  final String agentName;

  const ChatScreen({super.key, required this.agentName});

  @override
  ConsumerState<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends ConsumerState<ChatScreen> {
  final _scrollCtrl = ScrollController();

  void _scrollToBottom() {
    if (_scrollCtrl.hasClients) {
      Future.delayed(const Duration(milliseconds: 50), () {
        if (_scrollCtrl.hasClients) {
          _scrollCtrl.animateTo(
            _scrollCtrl.position.maxScrollExtent,
            duration: const Duration(milliseconds: 150),
            curve: Curves.easeOut,
          );
        }
      });
    }
  }

  @override
  void dispose() {
    _scrollCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final chatState = ref.watch(chatProvider(widget.agentName));
    final colors = RubixTokens.of(context);

    ref.listen(chatProvider(widget.agentName), (_, _) => _scrollToBottom());

    return Column(
      children: [
        // Minimal header
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            border: Border(
              bottom: BorderSide(color: colors.border, width: 0.5),
            ),
          ),
          child: Row(
            children: [
              InkWell(
                borderRadius: BorderRadius.circular(6),
                onTap: () => context.go('/agents'),
                child: Padding(
                  padding: const EdgeInsets.all(4),
                  child: Icon(LucideIcons.arrowLeft,
                      size: 18, color: colors.textSecondary),
                ),
              ),
              const SizedBox(width: 12),
              Text(
                widget.agentName,
                style: colors.inter(fontSize: 15, fontWeight: FontWeight.w500),
              ),
              const Spacer(),
              // New chat
              InkWell(
                borderRadius: BorderRadius.circular(6),
                onTap: () => ref
                    .read(chatProvider(widget.agentName).notifier)
                    .clearMessages(),
                child: Padding(
                  padding: const EdgeInsets.all(4),
                  child: Icon(LucideIcons.edit,
                      size: 18, color: colors.textSecondary),
                ),
              ),
            ],
          ),
        ),

        // Messages
        Expanded(
          child: chatState.messages.isEmpty
              ? _EmptyChat(agentName: widget.agentName, colors: colors)
              : ListView.builder(
                  controller: _scrollCtrl,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 24,
                    vertical: 16,
                  ),
                  itemCount: chatState.messages.length +
                      (chatState.isRunning ? 1 : 0),
                  itemBuilder: (context, i) {
                    // Thinking indicator at the end
                    if (i == chatState.messages.length) {
                      return _ThinkingIndicator(colors: colors);
                    }
                    final msg = chatState.messages[i];
                    return Center(
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 820),
                        child: _buildMessage(msg, colors),
                      ),
                    );
                  },
                ),
        ),

        // Input
        ChatInput(
          enabled: !chatState.isRunning,
          onSend: (text) =>
              ref.read(chatProvider(widget.agentName).notifier).send(text),
        ),
      ],
    );
  }

  Widget _buildMessage(ChatMessage msg, RubixColors colors) {
    switch (msg.type) {
      case ChatMessageType.userMessage:
      case ChatMessageType.text:
        return MessageBubble(message: msg);

      case ChatMessageType.toolCall:
        return ToolCallChip(toolName: msg.toolName ?? 'unknown');

      case ChatMessageType.connected:
        return const SizedBox.shrink(); // Don't show — too noisy

      case ChatMessageType.done:
        return Padding(
          padding: const EdgeInsets.only(top: 12, bottom: 4),
          child: Row(
            children: [
              Icon(LucideIcons.checkCircle,
                  size: 13, color: colors.textMuted),
              const SizedBox(width: 6),
              Text(
                '${msg.durationFormatted} · ${msg.costFormatted}',
                style: colors.mono(fontSize: 11, color: colors.textMuted),
              ),
            ],
          ),
        );

      case ChatMessageType.error:
        return Container(
          margin: const EdgeInsets.symmetric(vertical: 8),
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: RubixTokens.statusError.withValues(alpha: 0.08),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(
              color: RubixTokens.statusError.withValues(alpha: 0.2),
            ),
          ),
          child: Row(
            children: [
              Icon(LucideIcons.alertCircle,
                  size: 16, color: RubixTokens.statusError),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  msg.content,
                  style: colors.inter(
                      fontSize: 13, color: RubixTokens.statusError),
                ),
              ),
            ],
          ),
        );
    }
  }
}

/// Empty state — centered prompt, ChatGPT style.
class _EmptyChat extends StatelessWidget {
  final String agentName;
  final RubixColors colors;

  const _EmptyChat({required this.agentName, required this.colors});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: RubixTokens.accentCool.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Icon(
              LucideIcons.bot,
              size: 24,
              color: RubixTokens.accentCool,
            ),
          ),
          const SizedBox(height: 16),
          Text(
            agentName,
            style: colors.inter(fontSize: 18, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 6),
          Text(
            'How can I help you today?',
            style: colors.inter(fontSize: 14, color: colors.textMuted),
          ),
        ],
      ),
    );
  }
}

/// Animated thinking dots.
class _ThinkingIndicator extends StatelessWidget {
  final RubixColors colors;

  const _ThinkingIndicator({required this.colors});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 820),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 12),
          child: Row(
            children: [
              SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(
                  strokeWidth: 1.5,
                  color: colors.textMuted,
                ),
              ),
              const SizedBox(width: 10),
              Text(
                'Thinking...',
                style: colors.inter(fontSize: 13, color: colors.textMuted),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../../core/theme/markdown_theme.dart';
import '../models/chat_message.dart';

class MessageBubble extends StatelessWidget {
  final ChatMessage message;

  const MessageBubble({super.key, required this.message});

  @override
  Widget build(BuildContext context) {
    final colors = RubixTokens.of(context);
    final isUser = message.type == ChatMessageType.userMessage;

    if (isUser) {
      // User messages — right-aligned, subtle rounded container.
      return Align(
        alignment: Alignment.centerRight,
        child: Container(
          constraints: BoxConstraints(
            maxWidth: MediaQuery.of(context).size.width * 0.7,
          ),
          margin: const EdgeInsets.only(top: 20, bottom: 4),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: colors.surfaceBright,
            borderRadius: BorderRadius.circular(18),
          ),
          child: Text(
            message.content,
            style: colors.inter(fontSize: 15, height: 1.5),
          ),
        ),
      );
    }

    // AI messages — full-width, with small avatar, bare markdown.
    return Padding(
      padding: const EdgeInsets.only(top: 8, bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Avatar
          Container(
            width: 28,
            height: 28,
            margin: const EdgeInsets.only(top: 2, right: 12),
            decoration: BoxDecoration(
              color: RubixTokens.accentCool.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(7),
            ),
            child: Icon(
              LucideIcons.bot,
              size: 15,
              color: RubixTokens.accentCool,
            ),
          ),
          // Content
          Expanded(
            child: MarkdownBody(
              data: message.content,
              selectable: true,
              styleSheet: buildMarkdownTheme(context),
            ),
          ),
        ],
      ),
    );
  }
}

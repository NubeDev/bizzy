import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../application/qa_provider.dart';
import '../widgets/qa_answer_bubble.dart';
import '../widgets/qa_generating.dart';
import '../widgets/qa_question_card.dart';
import '../widgets/qa_result_view.dart';

class QaScreen extends ConsumerStatefulWidget {
  final String flow;
  final String title;

  const QaScreen({super.key, required this.flow, required this.title});

  @override
  ConsumerState<QaScreen> createState() => _QaScreenState();
}

class _QaScreenState extends ConsumerState<QaScreen> {
  final _scrollCtrl = ScrollController();

  @override
  void initState() {
    super.initState();
    // Start the QA flow after the first frame.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(qaProvider(widget.flow).notifier).start();
    });
  }

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
    final qa = ref.watch(qaProvider(widget.flow));
    final colors = RubixTokens.of(context);

    // Auto-scroll on state changes.
    ref.listen(qaProvider(widget.flow), (_, _) => _scrollToBottom());

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
                widget.title,
                style: colors.inter(fontSize: 15, fontWeight: FontWeight.w500),
              ),
            ],
          ),
        ),
        Expanded(
          child: SingleChildScrollView(
            controller: _scrollCtrl,
            padding: const EdgeInsets.symmetric(
              horizontal: Spacing.lg,
              vertical: Spacing.md,
            ),
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 820),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    // Completed exchanges
                    if (qa.exchanges.isNotEmpty) ...[
                      for (final exchange in qa.exchanges)
                        QaAnswerBubble(exchange: exchange),
                      const SizedBox(height: Spacing.sm),
                    ],

                    // Current question
                    if (qa.currentQuestion != null)
                      Padding(
                        padding:
                            const EdgeInsets.symmetric(vertical: Spacing.xs),
                        child: ConstrainedBox(
                          constraints: const BoxConstraints(maxWidth: 520),
                          child: QaQuestionCard(
                            field: qa.currentQuestion!,
                            onAnswer: (value) => ref
                                .read(qaProvider(widget.flow).notifier)
                                .answer(value),
                          ),
                        ),
                      ),

                    // Connecting
                    if (qa.phase == QaPhase.connecting)
                      const Center(
                        child: Padding(
                          padding: EdgeInsets.all(Spacing.xl),
                          child: RubixLoader(message: 'Connecting...'),
                        ),
                      ),

                    // Generating
                    if (qa.phase == QaPhase.generating)
                      Padding(
                        padding:
                            const EdgeInsets.symmetric(vertical: Spacing.xs),
                        child: QaGenerating(
                          message: qa.generatingMessage ?? 'Generating...',
                        ),
                      ),

                    // Streaming / done result — full width
                    if (qa.phase == QaPhase.streaming ||
                        qa.phase == QaPhase.done)
                      QaResultView(
                        qaState: qa,
                        onRestart: () => ref
                            .read(qaProvider(widget.flow).notifier)
                            .restart(),
                      ),

                    // Error
                    if (qa.phase == QaPhase.error)
                      Padding(
                        padding:
                            const EdgeInsets.symmetric(vertical: Spacing.sm),
                        child: RubixErrorState(
                          message: 'Something went wrong',
                          detail: qa.error,
                          onRetry: () => ref
                              .read(qaProvider(widget.flow).notifier)
                              .restart(),
                        ),
                      ),

                    // Streaming indicator
                    if (qa.phase == QaPhase.streaming)
                      Padding(
                        padding: const EdgeInsets.only(top: Spacing.sm),
                        child: Row(
                          children: [
                            const RubixLoader(size: 16),
                            const SizedBox(width: Spacing.xs),
                            Text(
                              'Streaming response...',
                              style: colors.inter(
                                fontSize: 12,
                                color: colors.textMuted,
                              ),
                            ),
                          ],
                        ),
                      ),

                    const SizedBox(height: Spacing.xl),
                  ],
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

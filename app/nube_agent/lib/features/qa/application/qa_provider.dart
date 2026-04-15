import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/api/nube_client.dart';
import '../../../core/auth/auth_repository.dart';
import '../../agents/application/auth_provider.dart';
import '../data/qa_ws_client.dart';
import '../models/qa_event.dart';
import '../models/qa_field.dart';

/// QA provider keyed by flow name (e.g. "nube-marketing.marketing_plan_qa").
final qaProvider =
    StateNotifierProvider.family<QaNotifier, QaSessionState, String>(
  (ref, flow) {
    final auth = ref.watch(authProvider).valueOrNull;
    return QaNotifier(flow: flow, credentials: auth);
  },
);

enum QaPhase {
  connecting,
  questioning,
  generating,
  streaming,
  done,
  error,
}

class QaSessionState {
  final QaPhase phase;
  final List<QaExchange> exchanges; // completed Q&A pairs
  final QaField? currentQuestion;
  final String? generatingMessage;
  final String resultText;
  final List<String> toolCalls;
  final String? sessionId;
  final String? model;
  final int? durationMs;
  final double? costUsd;
  final String? error;

  // Skill generator output (populated from "generating" event).
  final Map<String, dynamic>? createApp;
  final Map<String, dynamic>? createPrompt;
  final Map<String, dynamic>? createTool;
  final bool skillCreated;

  bool get hasSkillOutput => createApp != null;

  const QaSessionState({
    this.phase = QaPhase.connecting,
    this.exchanges = const [],
    this.currentQuestion,
    this.generatingMessage,
    this.resultText = '',
    this.toolCalls = const [],
    this.sessionId,
    this.model,
    this.durationMs,
    this.costUsd,
    this.error,
    this.createApp,
    this.createPrompt,
    this.createTool,
    this.skillCreated = false,
  });

  QaSessionState copyWith({
    QaPhase? phase,
    List<QaExchange>? exchanges,
    QaField? currentQuestion,
    String? generatingMessage,
    String? resultText,
    List<String>? toolCalls,
    String? sessionId,
    String? model,
    int? durationMs,
    double? costUsd,
    String? error,
    Map<String, dynamic>? createApp,
    Map<String, dynamic>? createPrompt,
    Map<String, dynamic>? createTool,
    bool? skillCreated,
  }) {
    return QaSessionState(
      phase: phase ?? this.phase,
      exchanges: exchanges ?? this.exchanges,
      currentQuestion: currentQuestion ?? this.currentQuestion,
      generatingMessage: generatingMessage ?? this.generatingMessage,
      resultText: resultText ?? this.resultText,
      toolCalls: toolCalls ?? this.toolCalls,
      sessionId: sessionId ?? this.sessionId,
      model: model ?? this.model,
      durationMs: durationMs ?? this.durationMs,
      costUsd: costUsd ?? this.costUsd,
      error: error ?? this.error,
      createApp: createApp ?? this.createApp,
      createPrompt: createPrompt ?? this.createPrompt,
      createTool: createTool ?? this.createTool,
      skillCreated: skillCreated ?? this.skillCreated,
    );
  }

  String get durationFormatted {
    if (durationMs == null) return '';
    return '${(durationMs! / 1000).toStringAsFixed(1)}s';
  }

  String get costFormatted {
    if (costUsd == null) return '';
    return '\$${costUsd!.toStringAsFixed(3)}';
  }
}

class QaNotifier extends StateNotifier<QaSessionState> {
  final String flow;
  final AuthCredentials? credentials;
  QaWsClient? _client;
  StreamSubscription<QaEvent>? _sub;

  QaNotifier({required this.flow, this.credentials})
      : super(const QaSessionState());

  /// Start the QA flow.
  void start() {
    if (credentials == null) return;

    state = const QaSessionState(phase: QaPhase.connecting);
    _client = QaWsClient(
      serverUrl: credentials!.serverUrl,
      token: credentials!.token,
    );

    _sub = _client!.connect(flow).listen(
      _handleEvent,
      onError: (e) {
        if (mounted) {
          state = state.copyWith(phase: QaPhase.error, error: e.toString());
        }
      },
      onDone: () {
        if (mounted && state.phase != QaPhase.done && state.phase != QaPhase.error) {
          state = state.copyWith(phase: QaPhase.done);
        }
      },
    );
  }

  void _handleEvent(QaEvent event) {
    if (!mounted) return;

    switch (event.type) {
      case QaEventType.session:
        state = state.copyWith(
          sessionId: event.sessionId,
          phase: QaPhase.connecting,
        );

      case QaEventType.question:
        state = state.copyWith(
          phase: QaPhase.questioning,
          currentQuestion: event.field,
        );

      case QaEventType.generating:
        state = state.copyWith(
          phase: QaPhase.generating,
          generatingMessage: event.message,
          currentQuestion: null,
          createApp: event.createApp,
          createPrompt: event.createPrompt,
          createTool: event.createTool,
        );

      case QaEventType.connected:
        state = state.copyWith(
          phase: QaPhase.streaming,
          model: event.model,
        );

      case QaEventType.toolCall:
        state = state.copyWith(
          toolCalls: [...state.toolCalls, event.toolName ?? 'unknown'],
        );

      case QaEventType.text:
        state = state.copyWith(
          resultText: state.resultText + (event.content ?? ''),
        );

      case QaEventType.done:
        state = state.copyWith(
          phase: QaPhase.done,
          durationMs: event.durationMs,
          costUsd: event.costUsd,
        );

      case QaEventType.error:
        state = state.copyWith(
          phase: QaPhase.error,
          error: event.error,
        );
    }
  }

  /// Submit an answer for the current question.
  void answer(dynamic value) {
    final q = state.currentQuestion;
    if (q == null || _client == null) return;

    // Record the exchange.
    final exchange = QaExchange(question: q, answer: value);
    state = state.copyWith(
      exchanges: [...state.exchanges, exchange],
      currentQuestion: null,
    );

    // Send to server.
    _client!.answer(value);
  }

  /// Create the skill from the generator output by calling the CRUD API.
  Future<void> createSkill() async {
    if (!state.hasSkillOutput || credentials == null) return;

    final client = NubeClient(
      baseUrl: credentials!.serverUrl,
      token: credentials!.token,
    );

    try {
      final appName = state.createApp!['name'] as String;

      await client.createApp(state.createApp!);

      if (state.createPrompt != null) {
        await client.createPrompt(appName, state.createPrompt!);
      }

      if (state.createTool != null) {
        await client.createTool(appName, state.createTool!);
      }

      await client.installApp(appName);

      if (mounted) {
        state = state.copyWith(skillCreated: true);
      }
    } catch (e) {
      if (mounted) {
        state = state.copyWith(error: 'Failed to create skill: $e');
      }
    }
  }

  /// Reset and start a new flow.
  void restart() {
    _sub?.cancel();
    _client?.close();
    start();
  }

  @override
  void dispose() {
    _sub?.cancel();
    _client?.close();
    super.dispose();
  }
}

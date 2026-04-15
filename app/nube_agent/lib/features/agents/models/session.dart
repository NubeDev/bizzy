class Session {
  final String id;
  final String agent;
  final String prompt;
  final String result;
  final String status;
  final int durationMs;
  final double costUsd;
  final String userId;
  final DateTime createdAt;

  const Session({
    required this.id,
    this.agent = '',
    this.prompt = '',
    this.result = '',
    this.status = '',
    this.durationMs = 0,
    this.costUsd = 0,
    this.userId = '',
    required this.createdAt,
  });

  factory Session.fromJson(Map<String, dynamic> json) {
    return Session(
      id: json['id'] as String? ?? '',
      agent: json['agent'] as String? ?? '',
      prompt: json['prompt'] as String? ?? '',
      result: json['result'] as String? ?? '',
      status: json['status'] as String? ?? '',
      durationMs: json['duration_ms'] as int? ?? 0,
      costUsd: (json['cost_usd'] as num?)?.toDouble() ?? 0,
      userId: json['user_id'] as String? ?? '',
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  String get durationFormatted {
    final secs = durationMs / 1000;
    return '${secs.toStringAsFixed(1)}s';
  }

  String get costFormatted => '\$${costUsd.toStringAsFixed(3)}';
}

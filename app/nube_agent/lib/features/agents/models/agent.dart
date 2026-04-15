class Agent {
  final String name;
  final String description;
  final List<AgentTool> tools;

  const Agent({
    required this.name,
    required this.description,
    this.tools = const [],
  });

  List<AgentTool> get qaTools => tools.where((t) => t.isQa).toList();
  bool get hasQaFlows => qaTools.isNotEmpty;

  factory Agent.fromJson(Map<String, dynamic> json) {
    return Agent(
      name: json['name'] as String? ?? '',
      description: json['description'] as String? ?? '',
      tools: (json['tools'] as List<dynamic>?)
              ?.map((t) => AgentTool.fromJson(t as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }
}

class AgentTool {
  final String name;
  final String type; // "openapi" or "js"
  final String description;
  final String mode; // "" or "qa"

  const AgentTool({
    required this.name,
    this.type = '',
    this.description = '',
    this.mode = '',
  });

  bool get isQa => mode == 'qa';

  /// Short display name: "nube-marketing.marketing_plan" → "marketing plan"
  String get shortName =>
      name.split('.').last.replaceAll('_qa', '').replaceAll('_', ' ');

  factory AgentTool.fromJson(Map<String, dynamic> json) {
    return AgentTool(
      name: json['name'] as String? ?? '',
      type: json['type'] as String? ?? '',
      description: json['description'] as String? ?? '',
      mode: json['mode'] as String? ?? '',
    );
  }
}

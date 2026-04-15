class StoreApp {
  final String id;
  final String name;
  final String displayName;
  final String description;
  final String longDescription;
  final String version;
  final String icon;
  final String color;
  final String category;
  final List<String> tags;
  final String authorId;
  final String authorName;
  final String workspaceId;
  final String visibility;
  final int installCount;
  final int activeInstalls;
  final double avgRating;
  final int reviewCount;
  final int toolCount;
  final int promptCount;
  final List<StoreTool> tools;
  final List<StorePrompt> prompts;
  final DateTime createdAt;
  final DateTime updatedAt;
  final DateTime? publishedAt;

  const StoreApp({
    required this.id,
    required this.name,
    this.displayName = '',
    this.description = '',
    this.longDescription = '',
    this.version = '1.0.0',
    this.icon = '',
    this.color = '',
    this.category = '',
    this.tags = const [],
    this.authorId = '',
    this.authorName = '',
    this.workspaceId = '',
    this.visibility = 'private',
    this.installCount = 0,
    this.activeInstalls = 0,
    this.avgRating = 0.0,
    this.reviewCount = 0,
    this.toolCount = 0,
    this.promptCount = 0,
    this.tools = const [],
    this.prompts = const [],
    required this.createdAt,
    required this.updatedAt,
    this.publishedAt,
  });

  /// Parse from the listing summary (no tools/prompts arrays, has toolCount/promptCount).
  factory StoreApp.fromSummary(Map<String, dynamic> json) {
    return StoreApp(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      displayName: json['displayName'] as String? ?? '',
      description: json['description'] as String? ?? '',
      version: json['version'] as String? ?? '',
      icon: json['icon'] as String? ?? '',
      color: json['color'] as String? ?? '',
      category: json['category'] as String? ?? '',
      tags: (json['tags'] as List<dynamic>?)?.cast<String>() ?? [],
      authorName: json['authorName'] as String? ?? '',
      installCount: json['installCount'] as int? ?? 0,
      avgRating: (json['avgRating'] as num?)?.toDouble() ?? 0.0,
      reviewCount: json['reviewCount'] as int? ?? 0,
      toolCount: json['toolCount'] as int? ?? 0,
      promptCount: json['promptCount'] as int? ?? 0,
      publishedAt: json['publishedAt'] != null
          ? DateTime.tryParse(json['publishedAt'] as String)
          : null,
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    );
  }

  /// Parse from full detail (has tools/prompts arrays).
  factory StoreApp.fromJson(Map<String, dynamic> json) {
    final tools = (json['tools'] as List<dynamic>?)
            ?.map((t) => StoreTool.fromJson(t as Map<String, dynamic>))
            .toList() ??
        [];
    final prompts = (json['prompts'] as List<dynamic>?)
            ?.map((p) => StorePrompt.fromJson(p as Map<String, dynamic>))
            .toList() ??
        [];

    return StoreApp(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      displayName: json['displayName'] as String? ?? '',
      description: json['description'] as String? ?? '',
      longDescription: json['longDescription'] as String? ?? '',
      version: json['version'] as String? ?? '',
      icon: json['icon'] as String? ?? '',
      color: json['color'] as String? ?? '',
      category: json['category'] as String? ?? '',
      tags: (json['tags'] as List<dynamic>?)?.cast<String>() ?? [],
      authorId: json['authorId'] as String? ?? '',
      authorName: json['authorName'] as String? ?? '',
      workspaceId: json['workspaceId'] as String? ?? '',
      visibility: json['visibility'] as String? ?? 'private',
      installCount: json['installCount'] as int? ?? 0,
      activeInstalls: json['activeInstalls'] as int? ?? 0,
      avgRating: (json['avgRating'] as num?)?.toDouble() ?? 0.0,
      reviewCount: json['reviewCount'] as int? ?? 0,
      toolCount: tools.length,
      promptCount: prompts.length,
      tools: tools,
      prompts: prompts,
      createdAt: json['createdAt'] != null
          ? DateTime.tryParse(json['createdAt'] as String) ?? DateTime.now()
          : DateTime.now(),
      updatedAt: json['updatedAt'] != null
          ? DateTime.tryParse(json['updatedAt'] as String) ?? DateTime.now()
          : DateTime.now(),
      publishedAt: json['publishedAt'] != null
          ? DateTime.tryParse(json['publishedAt'] as String)
          : null,
    );
  }

  bool get isPublic => visibility == 'public';
  bool get isPrivate => visibility == 'private';
}

class StoreTool {
  final String name;
  final String description;
  final String toolClass;
  final String mode;
  final String script;

  const StoreTool({
    required this.name,
    this.description = '',
    this.toolClass = 'read-only',
    this.mode = '',
    this.script = '',
  });

  factory StoreTool.fromJson(Map<String, dynamic> json) {
    return StoreTool(
      name: json['name'] as String? ?? '',
      description: json['description'] as String? ?? '',
      toolClass: json['toolClass'] as String? ?? '',
      mode: json['mode'] as String? ?? '',
      script: json['script'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() => {
        'name': name,
        'description': description,
        'toolClass': toolClass,
        if (mode.isNotEmpty) 'mode': mode,
        'params': <String, dynamic>{},
        'script': script,
      };
}

class StorePrompt {
  final String name;
  final String description;
  final String body;

  const StorePrompt({
    required this.name,
    this.description = '',
    this.body = '',
  });

  factory StorePrompt.fromJson(Map<String, dynamic> json) {
    return StorePrompt(
      name: json['name'] as String? ?? '',
      description: json['description'] as String? ?? '',
      body: json['body'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() => {
        'name': name,
        'description': description,
        'body': body,
      };
}

class AppReview {
  final String id;
  final String appId;
  final String userId;
  final String userName;
  final int rating;
  final String comment;
  final DateTime createdAt;

  const AppReview({
    required this.id,
    required this.appId,
    this.userId = '',
    this.userName = '',
    this.rating = 0,
    this.comment = '',
    required this.createdAt,
  });

  factory AppReview.fromJson(Map<String, dynamic> json) {
    return AppReview(
      id: json['id'] as String? ?? '',
      appId: json['appId'] as String? ?? '',
      userId: json['userId'] as String? ?? '',
      userName: json['userName'] as String? ?? '',
      rating: json['rating'] as int? ?? 0,
      comment: json['comment'] as String? ?? '',
      createdAt: json['createdAt'] != null
          ? DateTime.tryParse(json['createdAt'] as String) ?? DateTime.now()
          : DateTime.now(),
    );
  }
}

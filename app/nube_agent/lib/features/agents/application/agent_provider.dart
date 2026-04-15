import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/agent.dart';
import '../models/session.dart';
import 'auth_provider.dart';

/// List of available agents from the server.
final agentsProvider = FutureProvider<List<Agent>>((ref) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) return [];
  return client.listAgents();
});

/// List of past sessions.
final sessionsProvider = FutureProvider<List<Session>>((ref) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) return [];
  return client.listSessions();
});

/// Single session by ID.
final sessionDetailProvider =
    FutureProvider.family<Session, String>((ref, id) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) throw StateError('Not authenticated');
  return client.getSession(id);
});

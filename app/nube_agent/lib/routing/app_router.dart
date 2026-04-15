import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../features/agents/application/auth_provider.dart';
import '../features/admin/screens/admin_screen.dart';
import '../features/agents/screens/agent_detail_screen.dart';
import '../features/agents/screens/agents_screen.dart';
import '../features/auth/screens/login_screen.dart';
import '../features/chat/screens/chat_screen.dart';
import '../features/qa/screens/qa_screen.dart';
import '../features/sessions/screens/session_detail_screen.dart';
import '../features/sessions/screens/sessions_screen.dart';
import '../features/store/screens/store_screen.dart';
import '../features/store/screens/store_app_detail_screen.dart';
import '../features/store/screens/my_apps_screen.dart';
import '../features/store/screens/store_app_editor_screen.dart';
import 'app_shell.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final auth = ref.watch(authProvider);

  return GoRouter(
    initialLocation: '/agents',
    redirect: (context, state) {
      final loggedIn = auth.valueOrNull != null;
      final onLogin = state.matchedLocation == '/login';

      if (!loggedIn && !onLogin) return '/login';
      if (loggedIn && onLogin) return '/agents';
      return null;
    },
    routes: [
      GoRoute(
        path: '/login',
        builder: (context, state) => const LoginScreen(),
      ),

      // Shell with sidebar / bottom nav
      StatefulShellRoute.indexedStack(
        builder: (context, state, navigationShell) =>
            AppShell(navigationShell: navigationShell),
        branches: [
          // Branch 0: Agents
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/agents',
                builder: (context, state) => const AgentsScreen(),
              ),
              GoRoute(
                path: '/agent/:name',
                builder: (context, state) => AgentDetailScreen(
                  agentName: state.pathParameters['name']!,
                ),
              ),
              GoRoute(
                path: '/chat/:agent',
                builder: (context, state) => ChatScreen(
                  agentName: state.pathParameters['agent']!,
                ),
              ),
              GoRoute(
                path: '/qa/:flow',
                builder: (context, state) {
                  final flow = Uri.decodeComponent(
                      state.pathParameters['flow']!);
                  // Derive a display title from the flow name.
                  final title = flow
                      .split('.')
                      .last
                      .replaceAll('_qa', '')
                      .replaceAll('_', ' ');
                  return QaScreen(flow: flow, title: title);
                },
              ),
            ],
          ),

          // Branch 1: Sessions
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/sessions',
                builder: (context, state) => const SessionsScreen(),
              ),
              GoRoute(
                path: '/sessions/:id',
                builder: (context, state) => SessionDetailScreen(
                  sessionId: state.pathParameters['id']!,
                ),
              ),
            ],
          ),

          // Branch 2: Store
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/store',
                builder: (context, state) => const StoreScreen(),
              ),
              GoRoute(
                path: '/store/:id',
                builder: (context, state) => StoreAppDetailScreen(
                  appId: state.pathParameters['id']!,
                ),
              ),
              GoRoute(
                path: '/my-apps',
                builder: (context, state) => const MyAppsScreen(),
              ),
              GoRoute(
                path: '/my-apps/:id',
                builder: (context, state) => StoreAppEditorScreen(
                  appId: state.pathParameters['id']!,
                ),
              ),
            ],
          ),

          // Branch 3: Admin
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/admin',
                builder: (context, state) => const AdminScreen(),
              ),
            ],
          ),
        ],
      ),
    ],
  );
});

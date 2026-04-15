import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:rubix_ui/rubix_ui.dart';

import 'routing/app_router.dart';

class NubeAgentApp extends ConsumerWidget {
  const NubeAgentApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);

    return MaterialApp.router(
      title: 'Nube Agent',
      debugShowCheckedModeBanner: false,
      theme: RubixTokens.lightTheme,
      darkTheme: RubixTokens.darkTheme,
      themeMode: ThemeMode.dark,
      routerConfig: router,
    );
  }
}

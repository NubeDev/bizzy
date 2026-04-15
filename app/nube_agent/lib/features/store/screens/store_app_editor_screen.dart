import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../../agents/application/auth_provider.dart';
import '../application/store_provider.dart';
import '../models/store_app.dart';

/// Detail provider for the editor (fetches fresh from my/apps).
final _editorAppProvider =
    FutureProvider.family<StoreApp, String>((ref, id) async {
  final client = ref.watch(nubeClientProvider);
  if (client == null) throw Exception('Not authenticated');
  return client.getMyStoreApp(id);
});

class StoreAppEditorScreen extends ConsumerWidget {
  final String appId;

  const StoreAppEditorScreen({super.key, required this.appId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = RubixTokens.of(context);
    final appAsync = ref.watch(_editorAppProvider(appId));

    return Scaffold(
      backgroundColor: colors.bg,
      body: appAsync.when(
        loading: () => const Center(child: RubixLoader()),
        error: (e, _) => Center(
          child: RubixErrorState(
            message: 'Failed to load app',
            detail: e.toString(),
            onRetry: () => ref.invalidate(_editorAppProvider(appId)),
          ),
        ),
        data: (app) => _EditorBody(app: app, colors: colors),
      ),
    );
  }
}

class _EditorBody extends ConsumerWidget {
  final StoreApp app;
  final RubixColors colors;

  const _EditorBody({required this.app, required this.colors});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 700),
        child: ListView(
          padding: const EdgeInsets.all(24),
          children: [
            // Header
            Row(
              children: [
                RubixButton.icon(
                  icon: LucideIcons.arrowLeft,
                  onPressed: () => context.go('/my-apps'),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    app.displayName.isNotEmpty ? app.displayName : app.name,
                    style: colors.heading(fontSize: 22),
                  ),
                ),
                // Publish button
                if (!app.isPublic)
                  ElevatedButton.icon(
                    onPressed: () => _publish(context, ref),
                    icon: const Icon(LucideIcons.globe, size: 14),
                    label: const Text('Publish'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: const Color(0xFF10B981),
                      foregroundColor: Colors.white,
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8)),
                    ),
                  ),
                if (app.isPublic)
                  Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 12, vertical: 8),
                    decoration: BoxDecoration(
                      color:
                          const Color(0xFF10B981).withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Text('Published',
                        style: colors.inter(
                            fontSize: 13,
                            fontWeight: FontWeight.w600,
                            color: const Color(0xFF10B981))),
                  ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              'v${app.version}  |  ${app.visibility}  |  ${app.category.isEmpty ? "no category" : app.category}',
              style: colors.inter(fontSize: 13, color: colors.textMuted),
            ),

            const SizedBox(height: 24),

            // Metadata section
            _Section(
              title: 'Metadata',
              trailing: RubixButton.icon(
                icon: LucideIcons.pencil,
                onPressed: () => _editMetadata(context, ref),
              ),
              colors: colors,
              children: [
                _InfoRow('Name', app.name, colors),
                _InfoRow('Display Name', app.displayName, colors),
                _InfoRow('Description', app.description, colors),
                _InfoRow('Category', app.category.isEmpty ? '--' : app.category, colors),
                _InfoRow('Version', app.version, colors),
              ],
            ),

            const SizedBox(height: 24),

            // Tools section
            _Section(
              title: 'Tools (${app.tools.length})',
              trailing: RubixButton.icon(
                icon: LucideIcons.plus,
                onPressed: () => _addTool(context, ref),
              ),
              colors: colors,
              children: [
                if (app.tools.isEmpty)
                  Text('No tools yet.',
                      style: colors.inter(
                          fontSize: 13, color: colors.textMuted)),
                for (final tool in app.tools)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: Row(
                      children: [
                        Icon(LucideIcons.wrench,
                            size: 14, color: colors.textMuted),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(tool.name,
                                  style: colors.mono(
                                      fontSize: 13,
                                      fontWeight: FontWeight.w600)),
                              if (tool.description.isNotEmpty)
                                Text(tool.description,
                                    style: colors.inter(
                                        fontSize: 12,
                                        color: colors.textSecondary)),
                            ],
                          ),
                        ),
                        RubixButton.icon(
                          icon: LucideIcons.trash2,
                          onPressed: () => _deleteTool(context, ref, tool.name),
                        ),
                      ],
                    ),
                  ),
              ],
            ),

            const SizedBox(height: 24),

            // Prompts section
            _Section(
              title: 'Prompts (${app.prompts.length})',
              trailing: RubixButton.icon(
                icon: LucideIcons.plus,
                onPressed: () => _addPrompt(context, ref),
              ),
              colors: colors,
              children: [
                if (app.prompts.isEmpty)
                  Text('No prompts yet.',
                      style: colors.inter(
                          fontSize: 13, color: colors.textMuted)),
                for (final prompt in app.prompts)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: Row(
                      children: [
                        Icon(LucideIcons.fileText,
                            size: 14, color: colors.textMuted),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(prompt.name,
                                  style: colors.mono(
                                      fontSize: 13,
                                      fontWeight: FontWeight.w600)),
                              if (prompt.description.isNotEmpty)
                                Text(prompt.description,
                                    style: colors.inter(
                                        fontSize: 12,
                                        color: colors.textSecondary)),
                            ],
                          ),
                        ),
                        RubixButton.icon(
                          icon: LucideIcons.trash2,
                          onPressed: () =>
                              _deletePrompt(context, ref, prompt.name),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _refresh(WidgetRef ref) {
    ref.invalidate(_editorAppProvider(app.id));
    ref.invalidate(myStoreAppsProvider);
  }

  void _publish(BuildContext context, WidgetRef ref) async {
    final client = ref.read(nubeClientProvider);
    if (client == null) return;
    try {
      await client.publishStoreApp(app.id);
      _refresh(ref);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Publish failed: $e')));
      }
    }
  }

  void _editMetadata(BuildContext context, WidgetRef ref) {
    final displayCtrl = TextEditingController(text: app.displayName);
    final descCtrl = TextEditingController(text: app.description);
    final versionCtrl = TextEditingController(text: app.version);
    String category = app.category;
    final colors = RubixTokens.of(context);
    final cats = [
      '', 'iot-devices', 'analytics', 'devops', 'marketing',
      'design', 'utilities', 'integrations', 'automation'
    ];

    showDialog(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setState) => AlertDialog(
          backgroundColor: colors.surface,
          title:
              Text('Edit Metadata', style: colors.heading(fontSize: 18)),
          content: SizedBox(
            width: 400,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: displayCtrl,
                  style: colors.inter(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Display Name',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: descCtrl,
                  maxLines: 2,
                  style: colors.inter(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Description',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: versionCtrl,
                  style: colors.inter(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Version',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  value: category.isEmpty ? '' : category,
                  decoration: InputDecoration(
                    labelText: 'Category',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                  dropdownColor: colors.surface,
                  items: cats
                      .map((c) => DropdownMenuItem(
                            value: c,
                            child: Text(c.isEmpty ? 'None' : c),
                          ))
                      .toList(),
                  onChanged: (v) => setState(() => category = v ?? ''),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('Cancel'),
            ),
            ElevatedButton(
              onPressed: () async {
                final client = ref.read(nubeClientProvider);
                if (client == null) return;
                await client.updateStoreApp(app.id, {
                  'displayName': displayCtrl.text.trim(),
                  'description': descCtrl.text.trim(),
                  'version': versionCtrl.text.trim(),
                  'category': category,
                });
                _refresh(ref);
                if (context.mounted) Navigator.pop(context);
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: RubixTokens.accentCool,
                foregroundColor: Colors.black,
              ),
              child: const Text('Save'),
            ),
          ],
        ),
      ),
    );
  }

  void _addTool(BuildContext context, WidgetRef ref) {
    final nameCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    final scriptCtrl = TextEditingController();
    final colors = RubixTokens.of(context);

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: colors.surface,
        title: Text('Add Tool', style: colors.heading(fontSize: 18)),
        content: SizedBox(
          width: 500,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: nameCtrl,
                  style: colors.inter(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Name',
                    hintText: 'my-tool',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: descCtrl,
                  style: colors.inter(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Description',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: scriptCtrl,
                  maxLines: 8,
                  style: colors.mono(fontSize: 13),
                  decoration: InputDecoration(
                    labelText: 'JavaScript (handle function)',
                    hintText:
                        'function handle(params) {\n  return { message: "hello" };\n}',
                    hintStyle: colors.mono(
                        fontSize: 13, color: colors.textMuted),
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () async {
              final client = ref.read(nubeClientProvider);
              if (client == null) return;
              await client.addStoreTool(app.id, {
                'name': nameCtrl.text.trim(),
                'description': descCtrl.text.trim(),
                'toolClass': 'read-only',
                'params': <String, dynamic>{},
                'script': scriptCtrl.text,
              });
              _refresh(ref);
              if (context.mounted) Navigator.pop(context);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: RubixTokens.accentCool,
              foregroundColor: Colors.black,
            ),
            child: const Text('Add'),
          ),
        ],
      ),
    );
  }

  void _deleteTool(BuildContext context, WidgetRef ref, String name) async {
    final confirmed = await showRubixConfirm(
      context,
      title: 'Delete tool "$name"?',
      description: 'This cannot be undone.',
      confirmLabel: 'Delete',
      destructive: true,
    );
    if (confirmed != true) return;
    final client = ref.read(nubeClientProvider);
    if (client == null) return;
    await client.deleteStoreTool(app.id, name);
    _refresh(ref);
  }

  void _addPrompt(BuildContext context, WidgetRef ref) {
    final nameCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    final bodyCtrl = TextEditingController();
    final colors = RubixTokens.of(context);

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: colors.surface,
        title: Text('Add Prompt', style: colors.heading(fontSize: 18)),
        content: SizedBox(
          width: 500,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: nameCtrl,
                  style: colors.inter(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Name',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: descCtrl,
                  style: colors.inter(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Description',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: bodyCtrl,
                  maxLines: 8,
                  style: colors.mono(fontSize: 13),
                  decoration: InputDecoration(
                    labelText: 'Body (markdown with {{key}} substitution)',
                    border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () async {
              final client = ref.read(nubeClientProvider);
              if (client == null) return;
              await client.addStorePrompt(app.id, {
                'name': nameCtrl.text.trim(),
                'description': descCtrl.text.trim(),
                'body': bodyCtrl.text,
              });
              _refresh(ref);
              if (context.mounted) Navigator.pop(context);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: RubixTokens.accentCool,
              foregroundColor: Colors.black,
            ),
            child: const Text('Add'),
          ),
        ],
      ),
    );
  }

  void _deletePrompt(
      BuildContext context, WidgetRef ref, String name) async {
    final confirmed = await showRubixConfirm(
      context,
      title: 'Delete prompt "$name"?',
      description: 'This cannot be undone.',
      confirmLabel: 'Delete',
      destructive: true,
    );
    if (confirmed != true) return;
    final client = ref.read(nubeClientProvider);
    if (client == null) return;
    await client.deleteStorePrompt(app.id, name);
    _refresh(ref);
  }
}

// ── Helpers ──

class _Section extends StatelessWidget {
  final String title;
  final Widget? trailing;
  final RubixColors colors;
  final List<Widget> children;

  const _Section({
    required this.title,
    this.trailing,
    required this.colors,
    required this.children,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(title,
                style:
                    colors.inter(fontSize: 16, fontWeight: FontWeight.w600)),
            const Spacer(),
            if (trailing != null) trailing!,
          ],
        ),
        const SizedBox(height: 8),
        ...children,
      ],
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  final RubixColors colors;

  const _InfoRow(this.label, this.value, this.colors);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        children: [
          SizedBox(
            width: 120,
            child: Text(label,
                style: colors.inter(
                    fontSize: 13, color: colors.textSecondary)),
          ),
          Expanded(
            child: Text(
              value.isEmpty ? '--' : value,
              style: colors.inter(fontSize: 13),
            ),
          ),
        ],
      ),
    );
  }
}

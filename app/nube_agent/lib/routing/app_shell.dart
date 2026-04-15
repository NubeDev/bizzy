import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons/lucide_icons.dart';
import 'package:rubix_ui/rubix_ui.dart';

import '../features/agents/application/auth_provider.dart';

/// Responsive shell — clean sidebar on wide, minimal bottom bar on narrow.
class AppShell extends ConsumerWidget {
  final StatefulNavigationShell navigationShell;

  const AppShell({super.key, required this.navigationShell});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = RubixTokens.of(context);
    final isWide = MediaQuery.of(context).size.width > 800;

    if (isWide) {
      return Scaffold(
        backgroundColor: colors.bg,
        body: Row(
          children: [
            _Sidebar(
              selectedIndex: navigationShell.currentIndex,
              onTap: (i) => navigationShell.goBranch(i),
              onLogout: () => ref.read(authProvider.notifier).logout(),
              colors: colors,
            ),
            Expanded(child: navigationShell),
          ],
        ),
      );
    }

    return Scaffold(
      backgroundColor: colors.bg,
      body: navigationShell,
      bottomNavigationBar: _BottomBar(
        selectedIndex: navigationShell.currentIndex,
        onTap: (i) => navigationShell.goBranch(i),
        colors: colors,
      ),
    );
  }
}

class _Sidebar extends StatelessWidget {
  final int selectedIndex;
  final ValueChanged<int> onTap;
  final VoidCallback onLogout;
  final RubixColors colors;

  const _Sidebar({
    required this.selectedIndex,
    required this.onTap,
    required this.onLogout,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 260,
      decoration: BoxDecoration(
        color: colors.surface,
        border: Border(
          right: BorderSide(color: colors.border, width: 0.5),
        ),
      ),
      child: Column(
        children: [
          // Brand
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
            child: Row(
              children: [
                Container(
                  width: 28,
                  height: 28,
                  decoration: BoxDecoration(
                    color: RubixTokens.accentCool,
                    borderRadius: BorderRadius.circular(7),
                  ),
                  child: const Icon(
                    LucideIcons.bot,
                    size: 16,
                    color: Colors.black,
                  ),
                ),
                const SizedBox(width: 10),
                Text(
                  'Nube Agent',
                  style: colors.inter(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),

          // Nav
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Column(
              children: [
                _NavItem(
                  icon: LucideIcons.messageSquare,
                  label: 'Agents',
                  selected: selectedIndex == 0,
                  onTap: () => onTap(0),
                  colors: colors,
                ),
                const SizedBox(height: 2),
                _NavItem(
                  icon: LucideIcons.clock,
                  label: 'History',
                  selected: selectedIndex == 1,
                  onTap: () => onTap(1),
                  colors: colors,
                ),
                const SizedBox(height: 2),
                _NavItem(
                  icon: LucideIcons.settings,
                  label: 'Admin',
                  selected: selectedIndex == 2,
                  onTap: () => onTap(2),
                  colors: colors,
                ),
              ],
            ),
          ),

          const Spacer(),

          // Logout
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 16),
            child: _NavItem(
              icon: LucideIcons.logOut,
              label: 'Log out',
              selected: false,
              onTap: onLogout,
              colors: colors,
            ),
          ),
        ],
      ),
    );
  }
}

class _NavItem extends StatefulWidget {
  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;
  final RubixColors colors;

  const _NavItem({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
    required this.colors,
  });

  @override
  State<_NavItem> createState() => _NavItemState();
}

class _NavItemState extends State<_NavItem> {
  bool _hovered = false;

  @override
  Widget build(BuildContext context) {
    final bg = widget.selected
        ? widget.colors.surfaceBright
        : _hovered
            ? widget.colors.surfaceHover
            : Colors.transparent;

    return MouseRegion(
      onEnter: (_) => setState(() => _hovered = true),
      onExit: (_) => setState(() => _hovered = false),
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 120),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: bg,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            children: [
              Icon(
                widget.icon,
                size: 18,
                color: widget.selected
                    ? widget.colors.textPrimary
                    : widget.colors.textSecondary,
              ),
              const SizedBox(width: 12),
              Text(
                widget.label,
                style: widget.colors.inter(
                  fontSize: 14,
                  fontWeight:
                      widget.selected ? FontWeight.w500 : FontWeight.w400,
                  color: widget.selected
                      ? widget.colors.textPrimary
                      : widget.colors.textSecondary,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Minimal bottom bar — no Material BottomNavigationBar.
class _BottomBar extends StatelessWidget {
  final int selectedIndex;
  final ValueChanged<int> onTap;
  final RubixColors colors;

  const _BottomBar({
    required this.selectedIndex,
    required this.onTap,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: colors.surface,
        border: Border(
          top: BorderSide(color: colors.border, width: 0.5),
        ),
      ),
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Row(
            children: [
              _BottomTab(
                icon: LucideIcons.messageSquare,
                label: 'Agents',
                selected: selectedIndex == 0,
                onTap: () => onTap(0),
                colors: colors,
              ),
              _BottomTab(
                icon: LucideIcons.clock,
                label: 'History',
                selected: selectedIndex == 1,
                onTap: () => onTap(1),
                colors: colors,
              ),
              _BottomTab(
                icon: LucideIcons.settings,
                label: 'Admin',
                selected: selectedIndex == 2,
                onTap: () => onTap(2),
                colors: colors,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _BottomTab extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;
  final RubixColors colors;

  const _BottomTab({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: GestureDetector(
        onTap: onTap,
        behavior: HitTestBehavior.opaque,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              icon,
              size: 20,
              color: selected
                  ? colors.textPrimary
                  : colors.textMuted,
            ),
            const SizedBox(height: 4),
            Text(
              label,
              style: colors.inter(
                fontSize: 11,
                fontWeight: selected ? FontWeight.w500 : FontWeight.w400,
                color: selected
                    ? colors.textPrimary
                    : colors.textMuted,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

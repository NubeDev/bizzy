import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:rubix_ui/rubix_ui.dart';

/// Shared, polished markdown stylesheet used across chat and QA screens.
/// Designed to render AI-generated content cleanly at full width —
/// tables, code blocks, blockquotes, headings, lists.
MarkdownStyleSheet buildMarkdownTheme(BuildContext context) {
  final colors = RubixTokens.of(context);
  final isDark = Theme.of(context).brightness == Brightness.dark;

  final tableBorder = BorderSide(
    color: colors.border,
    width: 0.5,
  );

  return MarkdownStyleSheet(
    // ── Typography ──
    p: colors.inter(fontSize: 14.5, height: 1.7),
    pPadding: const EdgeInsets.only(bottom: 12),

    h1: colors.heading(fontSize: 26, height: 1.3),
    h1Padding: const EdgeInsets.only(top: 24, bottom: 12),

    h2: colors.heading(fontSize: 22, height: 1.3),
    h2Padding: const EdgeInsets.only(top: 20, bottom: 10),

    h3: colors.heading(fontSize: 18, height: 1.4),
    h3Padding: const EdgeInsets.only(top: 16, bottom: 8),

    h4: colors.inter(fontSize: 16, fontWeight: FontWeight.w600, height: 1.4),
    h4Padding: const EdgeInsets.only(top: 12, bottom: 6),

    h5: colors.inter(fontSize: 14, fontWeight: FontWeight.w600, height: 1.4),
    h5Padding: const EdgeInsets.only(top: 10, bottom: 4),

    strong: colors.inter(fontSize: 14.5, fontWeight: FontWeight.w700),
    em: colors.inter(
      fontSize: 14.5,
      fontWeight: FontWeight.w400,
      color: colors.textSecondary,
    ),

    // ── Lists ──
    listBullet: colors.inter(fontSize: 14.5, height: 1.7),
    listBulletPadding: const EdgeInsets.only(right: 8),
    listIndent: 24,

    // ── Code ──
    code: colors.mono(
      fontSize: 13,
      color: RubixTokens.accentCool,
      height: 1.5,
    ),
    codeblockDecoration: BoxDecoration(
      color: isDark
          ? const Color(0xFF1A1A1A)
          : const Color(0xFFF5F5F5),
      borderRadius: BorderRadius.circular(8),
      border: Border.all(color: colors.border, width: 0.5),
    ),
    codeblockPadding: const EdgeInsets.all(16),

    // ── Blockquote ──
    blockquoteDecoration: BoxDecoration(
      color: RubixTokens.accentCool.withValues(alpha: 0.05),
      border: Border(
        left: BorderSide(
          color: RubixTokens.accentCool,
          width: 3,
        ),
      ),
      borderRadius: const BorderRadius.only(
        topRight: Radius.circular(4),
        bottomRight: Radius.circular(4),
      ),
    ),
    blockquotePadding: const EdgeInsets.fromLTRB(16, 12, 16, 12),

    // ── Tables ──
    tableHead: colors.inter(
      fontSize: 13,
      fontWeight: FontWeight.w700,
      color: colors.textPrimary,
    ),
    tableBody: colors.inter(
      fontSize: 13,
      height: 1.5,
      color: colors.textSecondary,
    ),
    tableBorder: TableBorder(
      top: tableBorder,
      bottom: tableBorder,
      left: tableBorder,
      right: tableBorder,
      horizontalInside: tableBorder,
      verticalInside: tableBorder,
    ),
    tableHeadAlign: TextAlign.left,
    tableCellsPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
    tableColumnWidth: const FlexColumnWidth(),
    tableCellsDecoration: BoxDecoration(
      color: colors.surfaceWell.withValues(alpha: 0.3),
    ),

    // ── Horizontal rule ──
    horizontalRuleDecoration: BoxDecoration(
      border: Border(
        top: BorderSide(color: colors.border, width: 1),
      ),
    ),

    // ── Links ──
    a: colors.inter(
      fontSize: 14.5,
      color: RubixTokens.accentCool,
    ),
  );
}

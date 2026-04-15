# New Hardware Packing Prompt

Use this prompt when you need to generate a packing scope document for a hardware product.

## Goal

Create a practical markdown packing document in `dist/` for a new or existing hardware product.

Expected output path example:

- `dist/hardware/packaging/io-25-packing.md`

## What To Read First

- Product hardware reference docs in `docs/hardware/`
- Existing generated product scope in `dist/hardware/` if available
- PDF generation guide in [`usage.md`](/home/user/code/go/nube/developer-tools/docs/pdf-generator/usage.md)

## Q/A Workflow

Ask the user for only the missing packing details:

- Product name
- Product family
- Product category
- Unit packaging type
- Inner protection required
- Outer carton required yes or no
- Box label required yes or no
- Insert or quick-start card required yes or no
- Packed quantity per box
- Any accessories included in box
- SKU or model code
- Hardware revision required on box yes or no
- Country of origin text required yes or no
- Compliance text required yes or no
- Open items or approvals pending

If the AI tool supports a native UI for questions or feedback, use it.

## Output Structure

```markdown
# Product Name Packing Scope

## Summary

## Packing Type

## Required Packing Items

## Outer Box Printed Fields

## Packing Notes

## Open Items
```

## Rules

- Write output to `dist/`, not `docs/`
- Do not invent packaging dimensions or compliance text
- Use `TBD` for unknown values
- Keep the document short and implementation-focused
- If requested, show how to export the markdown to PDF with `doc2pdf`

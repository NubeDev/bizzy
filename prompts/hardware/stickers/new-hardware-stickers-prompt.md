# New Hardware Stickers Prompt

Use this prompt when you need to generate a sticker, barcode, or product labeling scope document for a hardware product.

## Goal

Create a practical markdown sticker and labeling document in `dist/` for a new or existing hardware product.

Expected output path example:

- `dist/hardware/stickers/io-25-stickers.md`

## What To Read First

- Product hardware reference docs in `docs/hardware/`
- Existing generated product scope in `dist/hardware/` if available
- PDF generation guide in [`usage.md`](/home/user/code/go/nube/developer-tools/docs/pdf-generator/usage.md)

## Q/A Workflow

Ask the user for only the missing label details:

- Product name
- Product family
- SKU or model number
- Label types needed
- Product identification sticker required yes or no
- Barcode label required yes or no
- Carton label required yes or no
- Barcode format, for example QR, Code 128, EAN
- Barcode payload or source field
- Serial number required yes or no
- MAC address required yes or no
- LoRa ID or provisioning code required yes or no
- Branding required yes or no
- Logo required yes or no
- Certification marks required
- Power or safety text required yes or no
- Label size constraints
- Label material or finish
- Open items or pending approvals

If the AI tool supports a native UI for questions or feedback, use it.

## Output Structure

```markdown
# Product Name Stickers and Labeling Scope

## Summary

## Label Types

## Required Printed Fields

## Barcode Requirements

## Compliance and Branding

## PCB and Silkscreen Reference Notes

## Open Items
```

## Rules

- Write output to `dist/`, not `docs/`
- Do not invent barcode payloads, serial schemes, or certification marks
- Use `TBD` for unknown values
- Reuse product identity fields from the hardware spec where possible
- Keep the document simple and review-ready
- If requested, show how to export the markdown to PDF with `doc2pdf`

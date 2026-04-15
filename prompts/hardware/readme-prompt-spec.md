# Hardware Main Prompt Spec

This document tells an AI assistant how to use hardware references and prompt files in this repository, and where to write generated output files.

## Purpose

Use this prompt folder for:

- AI prompt instructions
- hardware generation workflows
- executive summary workflows
- packaging and sticker Q/A workflows

## Folder Rules

- `docs/` is for real maintained documentation and stable reference docs
- `prompts/` is for AI prompts only
- `dist/` is for generated output files

Generated output should go to `dist/` so the user can review it, move it, rename it, or delete it later.

## Main Reference Files

- [`acbm.md`](/home/user/code/go/nube/developer-tools/docs/hardware/acbm.md) - ACBM gateway reference doc
- [`new-hardware-io-controller-prompt.md`](/home/user/code/go/nube/developer-tools/prompts/hardware/new-hardware-io-controller-prompt.md) - prompt for generating new I/O controller specs
- [`hardware-executive-summary-prompt.md`](/home/user/code/go/nube/developer-tools/prompts/hardware/hardware-executive-summary-prompt.md) - prompt for generating an executive summary
- [`new-hardware-packing-prompt.md`](/home/user/code/go/nube/developer-tools/prompts/hardware/packing/new-hardware-packing-prompt.md) - prompt for generating packing scope docs
- [`new-hardware-stickers-prompt.md`](/home/user/code/go/nube/developer-tools/prompts/hardware/stickers/new-hardware-stickers-prompt.md) - prompt for generating sticker and labeling scope docs
- [`usage.md`](/home/user/code/go/nube/developer-tools/docs/pdf-generator/usage.md) - PDF generation guide for `doc2pdf`
- [`devlopment.md`](/home/user/code/go/nube/developer-tools/docs/pdf-generator/devlopment.md) - developer notes for `doc2pdf`

## What The AI Should Do

When the user asks for a hardware executive summary:

1. Read the target hardware spec first.
2. Read related product family docs if they help explain context.
3. Identify:
   - what the product is
   - where it fits in the product family
   - the main customer or deployment use case
   - the most important connectivity, protocol, and power details
   - any major open items or risks
4. Write a short summary for an executive or stakeholder audience.
5. Ask the user whether they also want:
   - a packing document
   - a stickers and labeling document
6. If the AI tool supports a built-in UI for prompts, questions, or feedback, use it for this choice.
7. Save generated markdown in `dist/` under the matching hardware area.
8. If the user wants a PDF, use `doc2pdf` to export the markdown document.

## Paste-In Prompt For AI

Paste the block below into an AI tool when you want it to run a Q/A workflow and generate a hardware executive summary.

```markdown
You are helping generate a hardware executive summary from existing product documentation.

Follow this workflow:

1. Read the hardware docs provided by the user first.
2. Identify the product type, product family, use case, and main technical scope.
3. Run a short Q/A process to collect missing information before writing.
4. If the AI tool supports a built-in UI for questions or feedback, use it instead of dumping all questions in plain text.
   Examples include Claude Code VS Code and Codex VS Code extensions.
5. Ask the user whether they also want:
   - a packing document
   - a stickers and labeling document
6. Ask only the highest-value missing questions.
7. Do not invent specifications.
8. Mark unknown items as `TBD`.
9. After the Q/A, generate:
   - a short executive summary in markdown
   - optional packing doc if requested
   - optional stickers doc if requested
   - optional PDF export steps using `doc2pdf`

Use this Q/A checklist:

- Product name
- Product family
- Product category
- Main use case
- Target customer or deployment
- Processor or platform
- Key connectivity: Wi-Fi, Ethernet, LoRa, Thread, RS485
- Key protocols: BACnet, Modbus, MQTT, other
- Power method
- Main software stack
- Major differentiators
- Risks, gaps, or open items

Output format:

# Executive Summary: Product Name

## What It Is

## Where It Fits

## Key Capabilities

## Integration and Software

## Risks or Open Items

Keep the summary concise, professional, and useful for executive review.
Then, if requested, show how to export the markdown to PDF using:

go run ./cmd/doc2pdf <input.md> --output <output.pdf> --toc
```

## Q/A Rules For AI

- Ask questions in small batches, not as one giant form
- Prefer the AI tool's native question UI when available
- If the answer is already in the source docs, do not ask again
- Ask early whether the user wants packing and sticker docs included
- Ask only questions that materially improve the executive summary
- Separate confirmed facts from assumptions
- Keep the conversation moving toward a final markdown document

## Sticker, Barcode, and Silkscreen Q/A

Use this workflow when the user wants packaging, label, barcode, enclosure print, or PCB silkscreen documentation.

### What The AI Should Do

1. Read the related hardware product doc first.
2. Identify what needs artwork or labeling:
   - box label
   - product sticker
   - barcode label
   - carton label
   - PCB silkscreen
   - enclosure print
3. Run a short Q/A workflow to collect missing print and identification details.
4. Generate separate markdown output docs when needed, for example:
   - one packing document
   - one sticker or labeling document
5. If requested, export the markdown to PDF with `doc2pdf`.

### Paste-In Prompt For AI

Paste the block below into an AI tool when you want it to run a Q/A workflow for stickers, barcodes, or silkscreen.

```markdown
You are helping generate a hardware labeling document for stickers, barcodes, packaging, or silkscreen.

Follow this workflow:

1. Read the product hardware docs provided by the user first.
2. Identify the label or print type:
   - box sticker
   - unit product label
   - barcode label
   - shipping carton label
   - PCB silkscreen
   - enclosure print
3. Run a short Q/A process to collect missing information before writing.
4. If the AI tool supports a built-in UI for questions or feedback, use it instead of dumping all questions in plain text.
   Examples include Claude Code VS Code and Codex VS Code extensions.
5. Ask only the highest-value missing questions.
6. Do not invent regulatory, barcode, or certification data.
7. Mark unknown items as `TBD`.
8. After the Q/A, generate:
   - a concise markdown labeling or silkscreen scope document
   - optional PDF export steps using `doc2pdf`

Use this Q/A checklist:

- Product name
- Product family
- SKU or model number
- Label type
- Barcode type, for example Code 128, QR, EAN, serial label
- Barcode payload or barcode source field
- Serial number required yes or no
- MAC address label required yes or no
- LoRa ID or provisioning code required yes or no
- Certification marks required, for example CE, FCC, RCM, RoHS
- Company name and logo required yes or no
- Text fields required on the label
- Box contents text required yes or no
- Power rating text required yes or no
- Mounting or safety warning text required yes or no
- Silkscreen reference fields, for example terminal labels, polarity, RS485 A/B, power in, reset, LED names
- Color requirements
- Size constraints
- Material or print finish
- Open items or missing approvals

Output format:

# Labeling Scope: Product Name

## Summary

## Label Types

## Required Printed Fields

## Barcode Requirements

## Silkscreen Requirements

## Compliance and Branding

## Open Items

Keep the output concise, practical, and easy for engineering, operations, and design to review.
Then, if requested, show how to export the markdown to PDF using:

go run ./cmd/doc2pdf <input.md> --output <output.pdf> --toc
```

### Sticker and Silkscreen Q/A Rules

- Ask only for print-critical information
- Prefer the AI tool's native question UI when available
- Reuse product identity fields from the hardware spec where possible
- Do not guess certification logos or regulatory text
- Separate confirmed required print fields from optional ideas
- Keep the final document implementation-ready

### Suggested Output Structure

```markdown
# Labeling Scope: Product Name

## Summary

## Label Types

## Required Printed Fields

## Barcode Requirements

## Silkscreen Requirements

## Compliance and Branding

## Open Items
```

## Executive Summary Rules

- Keep it short and easy to scan
- Focus on product value and deployment role
- Mention hardware details only when they help a decision-maker
- Do not dump every spec line
- Do not invent missing details
- Use `TBD` or call out open items clearly
- Keep the tone direct and professional

## Suggested Executive Summary Structure

```markdown
# Executive Summary: Product Name

## What It Is

## Where It Fits

## Key Capabilities

## Integration and Software

## Risks or Open Items
```

## Good Inputs For The AI

The AI can use:

- finalized scope markdown docs
- early hardware specs
- PDF design documents converted into markdown notes
- product family reference docs

## PDF Generation Workflow

After generating the markdown summary:

1. Confirm the markdown file path.
2. Review the PDF generator usage guide:
   - [`usage.md`](/home/user/code/go/nube/developer-tools/docs/pdf-generator/usage.md)
3. Generate the PDF from the repo root with a command like:

```bash
go run ./cmd/doc2pdf docs/hardware/finalized-scope/io/14di-8ro.md --output dist/14di-8ro.pdf --toc
```

4. If the user wants HTML preview too:

```bash
go run ./cmd/doc2pdf docs/hardware/finalized-scope/io/14di-8ro.md --output dist/14di-8ro.pdf --html --toc
```

## PDF Rules For AI

- Use markdown as the source of truth
- Generate the executive summary in markdown first
- Then export with `doc2pdf`
- Check [`usage.md`](/home/user/code/go/nube/developer-tools/docs/pdf-generator/usage.md) for current flags and options
- Use frontmatter if title, author, date, or keywords are needed
- If diagrams are added later, review Mermaid support in the PDF docs first

## Folder Intent

- `docs/hardware/` contains shared hardware references
- `docs/hardware/finalized-scope/` contains more stable generated specs
- `prompts/hardware/` contains reusable prompt instructions
- `docs/pdf-generator/` contains the PDF export workflow and command examples

## Notes For Future Expansion

As more hardware types are added, keep separate finalized-scope folders for:

- gateways
- sensors
- io
- stickers

This makes it easier for AI tools to find a relevant example before generating a new summary or new hardware document.

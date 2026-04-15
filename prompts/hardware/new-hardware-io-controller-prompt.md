# New Hardware I/O Controller Prompt

---

## ⚠️ STOP - DO NOT EXPLORE ⚠️

**DO NOT do these things:**
- ❌ NO `git log`, `git show`, or git commands
- ❌ NO `find`, `ls`, or file exploration
- ❌ NO reading example files "for reference"
- ❌ NO reading docs.yaml
- ❌ NO browsing the repo

**Everything you need is in this prompt. Just extract → ask → generate.**

---

## Workflow (3 STEPS ONLY)

1. **EXTRACT** all info from user's request (product name, I/O counts, etc.)
2. **ASK** for missing critical info only (use question UI if available)
3. **GENERATE** spec and save to `dist/hardware/[product-name].md`

**Do NOT save to `docs/hardware/` or update `docs.yaml`**

## What to Ask (ONLY if not provided by user)

Ask in small batches using question UI:

1. **Product family?** (ACBM, ACBL)
2. **Deployment mode?** (side plugin / remote / both)
3. **Processor?** (STM32 / ESP32)
   - If STM32: skip wireless questions
   - If ESP32: ask about wireless only if remote mode
4. **Protocol?** (depends on deployment mode)

**Key rules:**
- Side plugin → non-ISO RS485, BACnet only, NO LoRa, powered by ACBM
- Remote I/O → ISO RS485, BACnet or Modbus, can have LoRa (if ESP32)
- STM32 → RS485 only, no wireless
- Don't ask for I/O counts if user already said them!

## Output Requirements

Generate a markdown document and save it to `dist/hardware/[product-name].md`.

The document should include these sections:

```markdown
# Product Name

## Summary

## Product Family

## Use Cases

## Hardware Platform

## Connectivity

## Power

## Field Interfaces

## I/O

## Protocols and Software

## Notes
```

## Writing Rules

**NO AI BLOAT - Factual specs only:**

- Keep the document simple and spec-focused
- Prefer short bullets and tables over long prose
- **Do not invent technical specifications:**
  - Don't invent relay voltage/current ratings
  - Don't invent sensor resistance values or signal ranges
  - Don't invent exact part numbers (use "STM32" not "STM32F103CBT6")
  - Don't invent processor specs (clock speed, flash size, RAM)
  - Don't invent compliance certifications (CE, FCC, UL)
  - Don't invent mechanical specs (dimensions, weight, IP rating)
- Use `TBD` for unknown items
- Call out contradictions in source material instead of guessing
- Keep marketing language out of the document
- **The spec should be a simple, factual outline - not a fake datasheet**

## Final Check

Before finishing, verify:

- The product family is named consistently
- Every yes or no capability is explicit
- I/O counts and signal types are easy to scan
- Software and protocol choices are separated clearly

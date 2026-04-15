# I/O Controller Spec Generator

---

## ⚠️ CRITICAL: DO NOT EXPLORE THE REPO ⚠️

**STOP. Do NOT do any of these things:**
- ❌ NO `git log` or `git show` commands
- ❌ NO `find` or `ls` commands to explore files
- ❌ NO reading example files "for reference"
- ❌ NO reading docs.yaml or other config files
- ❌ NO exploration of any kind

**You already have everything you need below. Just follow the 3-step workflow.**

---

## Workflow (ONLY DO THESE 3 STEPS)

### Step 1: EXTRACT from user's request

The user already told you key information. Extract it:
- Product name (e.g., "IO22")
- I/O counts: DI, DO, relay, UI, AO (use their EXACT numbers)
- Deployment mode (side plugin, remote, both)
- Any other details they mentioned

### Step 2: ASK for missing critical info ONLY

Use the built-in question UI (like Claude Code). Ask in small batches, ONLY for what's missing:
- Product family? (ACBM, ACBL)
- Deployment mode? (side plugin / remote I/O / both)
- Processor? (STM32 / ESP32)
- Protocol? (depends on deployment mode)

**Do NOT ask for info they already gave you!**

### Step 3: GENERATE and save

Create the markdown spec and save to: `dist/hardware/[product-name].md`

**Do NOT save to docs/hardware/ or update docs.yaml**

---

## Quick Reference: What to Ask

**Key constraints:**
- Side plugin → non-ISO RS485, BACnet only, NO LoRa, powered by ACBM
- Remote I/O → ISO RS485, BACnet or Modbus, can have LoRa (if ESP32)
- STM32 → RS485 only, no wireless
- ESP32 → can have wireless (if remote mode)

## Output Format

Save to: `dist/hardware/[product-name].md`

```markdown
# [Product Name]

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

## NO AI BLOAT - Keep Simple and Factual

**DO NOT invent technical specs:**

| ❌ Don't Write | ✅ Write Instead |
|---|---|
| "STM32F103CBT6, 72MHz, 128KB" | "STM32" |
| "Relay: 250VAC 10A SPDT" | "Relay output" |
| "10K NTC, 3950 beta" | "10K thermistor" |
| "CE, FCC Part 15, UL" | Omit or "TBD" |

Use `TBD` for unknowns. Keep it high-level.

---

## Example

User: "I want an IO22 with 12 DI and 10 DO"

**Extract:**
- Product name: IO22
- DI: 12
- DO: 10

**Ask (use question UI):**
- Product family?
- Deployment mode (side plugin / remote / both)?
- Processor (STM32 / ESP32)?

**Generate:** Create spec, save to `dist/hardware/io-22.md`

**DO NOT ask:** "How many DI?" or "Do you want 6 UI?" - they already told you!

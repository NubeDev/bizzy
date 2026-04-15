# Hardware Executive Summary Prompt

## Purpose

Use this prompt to turn a hardware specification into a short executive summary for management, partners, or customers who need a fast understanding of the product.

## Input

Provide:

- One hardware markdown spec
- Optional supporting files such as design notes, PDFs, or product family docs

## Workflow

1. Read the hardware spec first.
2. Identify the product category, target use case, and key differentiators.
3. Pull out only the highest-value technical points.
4. Summarize risks, gaps, or open decisions if they materially affect delivery.

## Output Requirements

Generate a markdown summary with this structure:

```markdown
# Executive Summary: Product Name

## What It Is

## Where It Fits

## Key Capabilities

## Integration and Software

## Delivery Risks or Open Items
```

## Writing Rules

- Keep it under 250 words unless the user asks for more
- Focus on business and product value, not low-level engineering detail
- Mention processor, connectivity, field protocols, power, and deployment role only if they matter to the decision-maker
- Do not copy large blocks from the source spec
- Do not invent roadmap or certification claims
- Use plain language

## Example Intent

A strong executive summary should quickly answer:

- What is this product
- Who is it for
- Why does it matter
- How does it connect into the broader system
- What still needs confirmation

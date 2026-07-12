# GoreGraph Dashboard Design System

## Product Character

Technical, calm, precise, dense enough for code navigation, and visually restrained. Decoration never competes with graph meaning.

## Tokens

- `--color-background: #f3f6f7`
- `--color-surface: #ffffff`
- `--color-canvas: #eef4f6`
- `--color-text: #17212b`
- `--color-muted: #5f6f7e`
- `--color-border: #d3dde4`
- `--color-accent: #0b6b79`
- `--color-focus: #0b6b79`
- `--color-success: #287a4b`
- `--color-warning: #a56a00`
- `--color-danger: #a33131`
- `--space-1: 4px`, `--space-2: 8px`, `--space-3: 12px`, `--space-4: 16px`, `--space-5: 20px`, `--space-6: 24px`
- `--radius-control: 6px`, `--radius-panel: 6px`
- no decorative shadows; inset selection indicators are functional

## Typography

Use the existing local-first Avenir Next / Segoe UI / Helvetica / Arial stack. Dashboard labels prioritize density and platform availability over brand display typography.

## Interaction

- Selection does not relayout the Architecture view.
- Selection does not center automatically.
- Isolation is explicit and reversible.
- Fit preserves search, filters, and selection.
- Focus indicators are visible on every interactive element.
- `prefers-reduced-motion` disables non-essential transitions.
- Inventory-like information uses semantic HTML at normal browser scale; it must never be shrunk to fit a large SVG viewBox.
- Pan, zoom, Fit, and relationship Labels are reserved for spatial graphs and implementation traces.
- Architecture focus uses redundant direction cues: outgoing is solid teal plus `OUT`, incoming is dashed amber plus `IN`, and both terminate at visible card-edge ports.
- Cards are opaque above background edges. A line that passes behind a card has no port; a line attached to a card ends at an explicit port.

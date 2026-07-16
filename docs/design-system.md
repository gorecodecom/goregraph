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
- Endpoints, Feature Flow, Data Flow, Diagnostics, and Coverage use dedicated semantic HTML workbenches. Coverage groups capability records by project and language instead of scaling the complete inventory into one graph.
- The six top-level views answer distinct questions: Architecture shows service relationships; Endpoints shows call paths; Feature Flow shows the implementation chain; Data Flow shows field-level movement; Diagnostics explains uncertain relationships; Coverage separates workspace completeness from analyzer support.
- At narrow widths the order is navigation, active workbench, then details. Multi-column verification, impact, and next-scan sections collapse to one column before text becomes cramped.
- Pan, zoom, Fit, and relationship Labels are reserved for spatial graphs and implementation traces.
- Implementation traces open at readable 100% card scale even when the complete path is wider or taller than the viewport. Pan explores the path; Fit is the explicit overview action.
- Architecture focus uses redundant direction cues: outgoing is solid teal, incoming is dashed amber, and both use arrowheads and terminate at visible card-edge ports. Text direction badges are omitted because the arrows already communicate direction.
- Cards are opaque above background edges. A line that passes behind a card has no port; a line attached to a card ends at an explicit port.
- Architecture uses dynamic domain lanes from service-map metadata, never a workspace-specific label or palette table.
- Service, domain, direction, and risk focus preserve every card coordinate and dim unrelated context.
- Background relations share domain/service trunks; selected direct relations fan out near opaque white cards and terminate at visible ports.
- The persistent relationship summary stays outside the SVG transform and complements, but never replaces, the inspector.
- `N calls` always means statically detected relationships, never runtime frequency; its tooltip is available on focus and hover.

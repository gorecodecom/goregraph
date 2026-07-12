# Readable Endpoints And Data Flow Design

## Problem

Endpoint Inventory and Data Flow currently render every matching record into one vertically growing SVG. The SVG viewBox then scales the entire inventory into the available canvas height. Text is therefore unreadable at the nominal 100% zoom, and increasing zoom does not provide a useful working view.

The inventory views are structured data workbenches, not spatial graphs. Treating them as graphs creates the wrong interaction model.

## Approved Direction

Use normal HTML workbench layouts for inventories and reserve SVG pan/zoom controls for actual relationship graphs and implementation traces.

The design follows `docs/design-system.md`: technical, calm, precise, restrained, dense but readable, with existing typography, colors, spacing, and focus behavior.

## Endpoint Inventory

- Render the inventory as a semantic, scrollable HTML table/list at normal browser scale.
- Use three stable columns: Caller, Endpoint, Provider.
- Keep route, status, relationship kind, and service names readable without zoom.
- Allow long routes and service names to wrap; do not shrink the entire inventory.
- Preserve the existing method, caller, provider, resolution, search, and risk filters.
- Selecting an endpoint opens the existing SVG implementation trace.
- Returning from a trace restores filters, selected service, and inventory scroll position without changing scale.
- The initial state may show all currently matching relationships, but the list scrolls inside the main workbench rather than expanding the SVG coordinate system.
- Keyboard activation and visible focus are required for every selectable row.

## Data Flow

- Keep the searchable Data Flow list in the left sidebar as the master list.
- Do not render every flow simultaneously in the main canvas.
- With no selection, show a readable explanation asking the user to choose a flow and explaining what the view reveals.
- Selecting a flow renders only that flow in the main workbench.
- Present its nodes as a readable ordered chain at 100% browser scale.
- Use horizontal flow on sufficiently wide canvases and vertical flow on narrow canvases.
- Show node role, label/field, confidence, and source summary.
- Render explicit gaps at the correct position as semantic warning blocks, never as inferred facts.
- Selecting a node shows full symbol, file/line, evidence, confidence, and available trace actions in the existing details area.
- Selection and filtering do not automatically move, zoom, or relayout unrelated views.

## Zoom And Toolbar Rules

- Architecture and implementation traces retain Minus, Plus, 100%, Fit, Labels, and zoom readout as applicable.
- Endpoint Inventory and Data Flow HTML workbenches hide graph zoom controls.
- Opening an endpoint trace restores graph controls.
- Returning to inventory restores the readable HTML layout and its scroll position.
- Diagnostics and Coverage remain unchanged in this focused fix.

## Architecture Direction Clarity

- Preserve the full Architecture Map and its stable layout during ordinary selection.
- Keep all unselected relationships visible but strongly subdued so they provide context without competing with the selected neighborhood.
- Relative to the selected service, render outgoing relationships in the existing teal accent with a solid stroke.
- Render incoming relationships in the existing warning/amber role color with a restrained dashed stroke.
- Increase focused arrowhead size and contrast, and end the path before the target card so the arrowhead remains visible instead of disappearing under the card border.
- Route every connected edge to an explicit card-edge port. A small visible port dot marks the exact source and target attachment point.
- Render cards as fully opaque surfaces above background edges. A relationship that merely passes behind a card is masked and has no port, while a relationship attached to the card terminates at its visible port.
- Keep a short, unobstructed terminal segment between the final curve and each port so crossings near the card cannot be mistaken for an attachment.
- Choose the nearest sensible card side for each port from the relative node positions; do not force a line through the card interior.
- Separate multiple focused connections at one card into vertically distributed ports instead of stacking every arrowhead on one indistinguishable point.
- Add a small semantic `OUT` badge to connected target cards and an `IN` badge to connected source cards. The badge describes the relationship relative to the selected service, not the global project type.
- When a connected node has both incoming and outgoing relationships, show both compact badges.
- Do not encode direction by color alone: stroke style, arrowhead, and the text badge provide redundant cues.
- Keep optional edge labels limited to focused relationships; labels must not be required to understand direction.
- The details panel continues to list incoming and outgoing relationships textually for exact inspection.

## Responsive Behavior

- Desktop Endpoint Inventory uses three columns.
- Narrow layouts stack Caller, Endpoint, and Provider inside each row while preserving their labels.
- Data Flow switches from horizontal to vertical chain before cards become cramped.
- The sidebar and main content retain independent scrolling behavior.
- No text is scaled below the dashboard design-system type sizes.

## Accessibility

- Endpoint rows and Data Flow selections are real buttons or keyboard-operable rows with descriptive accessible names.
- The selected row uses both visual state and `aria-pressed` or `aria-current` semantics.
- Empty states and filter result counts remain announced text.
- Existing focus tokens and reduced-motion behavior remain in force.

## State And Compatibility

- No JSON schema or generated data contract changes are required.
- Existing endpoint filters, selected service, selected trace, trace-from-here behavior, Data Flow records, search, and details remain compatible.
- New UI state is limited to selected Data Flow ID and saved inventory scroll positions.
- Workspace output remains a standalone offline HTML file.

## Testing And Acceptance

- Add renderer contract tests proving inventories no longer generate one SVG row per record.
- Assert Endpoint Inventory uses the HTML workbench, retains all filters, and opens the existing trace.
- Assert Data Flow initially shows guidance and renders exactly one selected flow.
- Assert toolbar visibility follows the view/trace rules.
- Assert long route text wraps and interactive rows are keyboard accessible.
- Assert selected Architecture relationships receive distinct incoming/outgoing classes, explicit source/target ports, unobstructed target arrowheads, opaque card masking, and textual direction badges without changing node positions.
- Run the full Go test suite and `go vet`.
- Install the resulting local version, clean and rescan all 44 WEKA projects, then inspect the generated HTML contract.
- Browser testing remains excluded unless the user explicitly reverses the earlier no-browser instruction; residual visual risk must be reported.

# Dashboard Navigation and Monorepo Layout Design

**Status:** Approved for planning
**Date:** 2026-07-22
**Release target:** Unreleased `1.3.0`

## Purpose

Improve the generated workspace dashboard in four related areas without changing its offline-first model:

1. make Code Explorer package groups collapsible;
2. let users independently hide and restore the left and right dashboard panels;
3. make the existing writable Architecture layout editor discoverable from the normal dashboard;
4. prevent an internal package of a multi-package monorepo from defining the architecture group of the entire repository.

The repository or discovered project root remains the service boundary. Internal applications, packages, and modules contribute analysis evidence but do not become additional services.

## Scope

### Collapsible Code Explorer groups

- Render every Code Explorer package or module group with an accessible toggle button and a symbol count.
- Collapse groups by default.
- Keep the group containing the selected symbol open.
- While a search is active, automatically open groups that contain matching symbols.
- Preserve explicit expand and collapse choices for the current Code Explorer session.
- Support pointer, keyboard, and screen-reader interaction through native button semantics and `aria-expanded` state.
- Preserve the current symbol selection, usage filters, source actions, and direct-versus-API evidence behavior.

### Persistent dashboard panels

- Add independent controls for the left navigation panel and right details panel.
- Keep both controls reachable when their corresponding panel is hidden.
- Expand the main canvas or workbench into the released space without changing the active dashboard view.
- Keep both panels visible by default for a workspace with no saved preference.
- Persist only the two panel visibility preferences in browser local storage, scoped to the workspace identity and dashboard schema.
- Treat local-storage access as optional. If it is unavailable, the dashboard remains functional and uses visible panels.
- Update accessibility state when a panel changes so hidden content is not keyboard-focusable or exposed as active content.

### Discoverable Architecture editing

- Keep the existing Architecture editor implementation and its Save, Discard, Reset, conflict, validation, drag-and-drop, and keyboard-equivalent behavior.
- Show the `Edit layout` action in both static and editor-enabled dashboards.
- In an editor-enabled localhost session, open the existing writable editor directly.
- In a static dashboard, open an explanatory dialog instead of pretending that a local HTML file can write configuration.
- Show the command `goregraph dashboard edit .` in that dialog and provide a copy action.
- Add `goregraph dashboard edit <path>` as a concise alias that resolves the workspace and delegates to the existing secure workspace dashboard editor.
- Retain the existing loopback-only server, session token, origin and host validation, optimistic revision handling, and atomic `.goregraph-dashboard.json` writes.
- Document the distinction between static viewing and writable editing in global help, command help, `COMMANDS.md`, and `README.md`.

### Multi-package monorepo grouping

- Continue discovering one project at the repository or accepted project-marker boundary. Nested package-manager workspaces, applications, and build modules do not become separate workspace services.
- Detect when one discovered project contains multiple internal production packages or modules using supported package-manager and build metadata together with indexed production evidence.
- For such a multi-package project, do not select one internal package or module as the architecture group for the entire service.
- Use the existing repository-path fallback group instead. In a layout such as `frontend/frontend-monorepo`, the resulting group label is `frontend`.
- Continue using a coherent production package namespace for a single-package project where that namespace represents the whole project.
- Preserve current Java package-family grouping and other single-package behavior unless the project is demonstrably multi-package.
- Preserve deterministic ordering and manual `.goregraph-dashboard.json` precedence.
- Show whether a group or service assignment came from repository-boundary evidence, production package evidence, or manual configuration in the Architecture editor.

For the representative Weka workspace, `frontend`, `frontend-monorepo`, `frontends`, and `playwright` therefore resolve to the `frontend` architecture group while remaining four independent services.

### Worktree exclusion

- Add `.worktrees` to the recursive default project-scan exclusions, not only workspace project discovery exclusions.
- Never index files below `.worktrees`, even when that directory occurs inside an accepted project root.
- Do not modify or delete a user's `.worktrees` directory.
- Remove previously indexed worktree facts naturally when the project and workspace projections are rebuilt.
- Do not add a dependency or require Python for this behavior.

## Interaction Design

The main dashboard keeps its current three-column desktop structure. Slim panel toggles sit at the left and right edges of the main area so they remain available after a panel collapses. Toggling a panel changes the grid layout rather than overlaying an invisible panel over the canvas. Existing responsive stacking remains usable at narrow widths, with the same controls and saved state.

Code Explorer group headings become compact disclosure rows. The toggle contains the package or module name and its result count. The selected symbol's group is opened before rendering so selection never becomes invisible. Search-driven expansion is derived from the filtered result set and does not permanently overwrite explicit choices made outside the active search.

The Architecture `Edit layout` action remains in the existing Architecture view toolbar. Static mode presents a small dialog with the exact editor command, a copy button, and a close button. Editor-enabled mode skips the dialog and enters the existing editor. The command alias keeps the common workflow short while preserving the current security boundary: writable changes still require an explicit local editor process.

## Data and State Flow

### Panel preferences

1. Dashboard startup derives a storage key from the workspace root and dashboard schema.
2. It attempts to read a small versioned value containing `leftVisible` and `rightVisible` booleans.
3. Missing, malformed, or inaccessible storage falls back to both panels visible.
4. A toggle updates the DOM, accessibility attributes, layout class, and stored value.
5. Panel state does not enter generated GoreGraph output or `.goregraph-dashboard.json`.

### Code Explorer disclosure state

1. The dashboard groups the currently filtered symbols by the existing package/module grouping key.
2. A session-local set tracks explicit disclosure choices.
3. The selected symbol's group is forced open.
4. Active search results are opened so matching symbols remain discoverable.
5. Leaving or re-rendering Code Explorer preserves session choices without creating long-lived browser configuration.

### Architecture configuration

Static dashboards remain read-only. The editor command starts the existing loopback server, opens the same generated dashboard with an authenticated editor runtime, loads the current revision, and writes validated configuration atomically. Manual configuration continues to override detected grouping during reconciliation.

### Monorepo evidence

Grouping first determines whether the discovered project represents multiple internal production packages or modules. When it does, the group resolver uses the project path fallback and records repository-boundary provenance. When it does not, the existing coherent production namespace resolver may select a package-derived group. The decision is deterministic and independent of symbol or project discovery order.

## Error Handling

- Local-storage exceptions, malformed values, and unavailable storage never block dashboard startup or toggling.
- A static dashboard never exposes a save control that cannot persist changes.
- Clipboard failure leaves the editor command visible and selectable and reports a concise message.
- Missing or invalid editor configuration continues to use the existing field-specific validation and Doctor reporting.
- Editor process loss preserves the browser draft and reports the existing offline state.
- Ambiguous multi-package evidence falls back to the repository path rather than choosing an arbitrary internal package.
- A failed rebuild does not delete the last successfully generated dashboard.

## Compatibility

- The generated dashboard remains a self-contained offline HTML application.
- No new runtime, frontend framework, Python installation, or third-party dependency is introduced.
- Existing view selection, filters, graph zoom and pan, source actions, inspector behavior, editor configuration, and generated schemas remain compatible.
- The implementation uses Go path operations and browser-standard APIs without Windows- or macOS-specific shell assumptions.
- The editor continues to bind only to loopback and requires its generated session token.
- No release tag, package publication, or hosted service is part of this work.

## Testing

Every behavior change starts with a failing test.

### Grouping and scanning

- A multi-package JavaScript/TypeScript workspace remains one service and uses its repository-path group.
- A single-package frontend project retains coherent package grouping where applicable.
- Java package-family grouping remains unchanged for single-service projects.
- Manual service and group overrides remain authoritative.
- Grouping is stable across input permutations.
- Nested `.worktrees` content is excluded from project files, symbols, relations, and downstream workspace projections.
- Existing project discovery exclusions remain intact.

### Dashboard behavior

- Code Explorer groups render collapsed with accessible names, counts, and `aria-expanded` state.
- Selecting a symbol keeps its group open.
- Search opens matching groups without losing session disclosure choices.
- Left and right panels toggle independently and update layout and accessibility state.
- Valid persisted panel state is restored; invalid or inaccessible storage falls back safely.
- Static Architecture editing shows the command dialog and copy fallback.
- Editor-enabled Architecture editing still opens the writable editor.
- `goregraph dashboard edit .` resolves a workspace and starts the established editor path.

### Verification

- Run focused unit and dashboard-script tests first.
- Run the affected `internal/scan`, `internal/cli`, and `internal/dashboardeditor` packages.
- Run `go vet` for the affected packages and format changed Go files with `gofmt`.
- Generate and inspect representative dashboards at desktop widths, including 1280, 1440, and 1920 pixels.
- Rebuild the Weka workspace with the locally installed unreleased `1.3.0` binary and verify the four frontend services, `.worktrees` exclusion, panel persistence, Code Explorer disclosure behavior, and writable editor entry.

## Acceptance Criteria

1. Package groups in Code Explorer can be expanded and collapsed independently and are collapsed by default without hiding the current selection or search matches.
2. Left and right dashboard panels can be hidden independently, stay hidden across reopening the same workspace dashboard, and can always be restored.
3. Architecture editing is visible from the normal dashboard and gives a direct, truthful route to the writable editor.
4. `goregraph dashboard edit .` starts the existing secure editor workflow for a workspace.
5. A multi-package monorepo remains one service and is not grouped by one dominant internal package.
6. The representative Weka frontend repositories appear under the `frontend` architecture group unless a manual override says otherwise.
7. `.worktrees` content is absent from new project and workspace projections without modifying the directory itself.
8. Existing static dashboard, single-package grouping, Java grouping, manual configuration, and editor-security behavior remain compatible.
9. The affected regression, vet, generated-dashboard, and visual checks pass on the unreleased `1.3.0` source without creating a release or publication.

package scan

import (
	"html"
	"strings"
)

const workspaceDashboardShell = `<div class="shell">
<aside class="side">
<h1>GoreGraph Workspace</h1>
<div class="summary"><div class="metric"><strong id="service-count">0</strong><span>services</span></div><div class="metric"><strong id="edge-count">0</strong><span>relations</span></div><div class="metric"><strong id="trace-count">0</strong><span>traces</span></div><div class="metric"><strong id="contract-count">0</strong><span>contracts</span></div></div>
<h2>View</h2>
<div class="modes">
<button data-view-mode="architecture" class="active" aria-pressed="true">Architecture</button>
<button data-view-mode="api-catalog" data-mode-help="Browse the canonical API inventory by provider without changing endpoint trace filters." aria-pressed="false">API Catalog</button>
<button data-view-mode="endpoints" aria-pressed="false">Endpoints</button>
<button data-view-mode="feature-flow" aria-pressed="false">Feature Flow</button>
<button data-view-mode="data-flow" aria-pressed="false">Data Flow</button>
<button data-view-mode="code-explorer" aria-pressed="false">Code Explorer</button>
<button data-view-mode="diagnostics" aria-pressed="false">Diagnostics</button>
<button data-view-mode="coverage" aria-pressed="false">Coverage</button>
</div>
<p class="help" id="mode-help">See how projects and services communicate across the workspace. Select a service to highlight direct incoming and outgoing relationships without changing the layout.</p>
<input id="workspace-search" aria-label="Search workspace map" placeholder="Search service, endpoint, route, file, symbol">
<section id="workspace-kind-controls">
<h2>Filter</h2>
<div class="filters">
<button data-kind-filter="all" class="active" aria-pressed="true">All</button>
<button data-kind-filter="risk" aria-pressed="false">Risk</button>
<button data-kind-filter="resolved" aria-pressed="false">Resolved</button>
<button data-kind-filter="unresolved" aria-pressed="false">Unresolved</button>
</div>
<div class="filters" style="margin-top:8px"><button id="clear-selection" type="button">Clear selection</button></div>
</section>
<div class="filters" style="margin-top:8px"><button id="isolate-neighborhood" type="button" hidden>Isolate neighborhood</button><button id="show-full-architecture" type="button" hidden>Show full architecture</button></div>
<div class="filters" style="margin-top:8px"><button id="focus-selected" type="button" hidden>Focus selected</button><button id="back-to-full-architecture" type="button" hidden>Back to full architecture</button></div>
<section id="api-catalog-filters" class="endpoint-filters" hidden aria-label="API Catalog filters">
<h2>API Catalog</h2>
<p class="help">Browse the canonical API inventory by provider without changing endpoint trace filters.</p>
<label class="filter-label" for="api-catalog-provider-filter">Provider service</label><select id="api-catalog-provider-filter" aria-label="Filter API Catalog provider service"></select>
<label class="filter-label" for="api-catalog-search">Endpoint text</label><input id="api-catalog-search" type="search" aria-label="Filter API Catalog endpoint text" placeholder="Search path, handler, or source">
<div class="filters api-catalog-methods" aria-label="API Catalog HTTP methods"><button type="button" data-api-catalog-method="GET" aria-pressed="false">GET</button><button type="button" data-api-catalog-method="POST" aria-pressed="false">POST</button><button type="button" data-api-catalog-method="PUT" aria-pressed="false">PUT</button><button type="button" data-api-catalog-method="PATCH" aria-pressed="false">PATCH</button><button type="button" data-api-catalog-method="DELETE" aria-pressed="false">DELETE</button><button type="button" data-api-catalog-method="OTHER" aria-pressed="false">Other</button></div>
<label class="filter-label" for="api-catalog-security-filter">Endpoint security</label><select id="api-catalog-security-filter" multiple aria-label="Filter API Catalog endpoint security"></select>
<label class="filter-label" for="api-catalog-consumer-filter">Consumers</label><select id="api-catalog-consumer-filter" multiple aria-label="Filter API Catalog consumers"></select>
<label class="filter-label" for="api-catalog-status-filter">Status</label><select id="api-catalog-status-filter" multiple aria-label="Filter API Catalog status"></select>
<button id="clear-api-catalog-filters" type="button">Clear filters</button><p id="api-catalog-filter-summary" aria-live="polite"></p>
</section>
<section id="endpoint-filters" class="endpoint-filters" hidden aria-label="Endpoint debugging filters">
<h2>Endpoint debugging</h2>
<div class="filters endpoint-methods" aria-label="HTTP methods"><button type="button" data-endpoint-method="GET" aria-pressed="false">GET</button><button type="button" data-endpoint-method="POST" aria-pressed="false">POST</button><button type="button" data-endpoint-method="PUT" aria-pressed="false">PUT</button><button type="button" data-endpoint-method="PATCH" aria-pressed="false">PATCH</button><button type="button" data-endpoint-method="DELETE" aria-pressed="false">DELETE</button><button type="button" data-endpoint-method="OTHER" aria-pressed="false">Other</button></div>
<label class="filter-label" for="endpoint-caller-filter">Caller services</label><select id="endpoint-caller-filter" multiple aria-label="Filter caller services"></select>
<label class="filter-label" for="endpoint-provider-filter">Provider services</label><select id="endpoint-provider-filter" multiple aria-label="Filter provider services"></select>
<div class="filters endpoint-statuses" aria-label="Endpoint status"><button type="button" data-endpoint-status="resolved" aria-pressed="false">Resolved</button><button type="button" data-endpoint-status="unresolved" aria-pressed="false">Unresolved</button><button type="button" data-endpoint-status="mismatch" aria-pressed="false">Mismatch</button><button type="button" data-endpoint-status="out_of_scope" aria-pressed="false">Out of scope</button></div>
<button id="clear-endpoint-filters" type="button">Clear filters</button><p id="endpoint-filter-summary" aria-live="polite"></p>
</section>
<h2 id="list-title">Services</h2>
<div id="node-list" class="item-list"></div>
<p id="result-note" class="result-note" aria-live="polite"></p>
<div class="glossary">
<h2>Status glossary</h2>
<p><strong>RESOLVED</strong>: frontend contract is mapped to a backend route.</p>
<p><strong>MISMATCH</strong>: a related backend route exists, but method or path does not match exactly.</p>
<p><strong>UNRESOLVED</strong>: no reliable backend route was found in indexed projects.</p>
<p><strong>OUT_OF_SCOPE</strong>: intentionally not a backend service, for example a frontend-internal API.</p>
<p><strong>EXTRACTED</strong>: read directly from source structure. <strong>MATCHED</strong>: linked to a concrete route, symbol, or test.</p>
</div>
</aside>
<main>
<nav id="architecture-view-tabs" class="architecture-tabs" aria-label="Architecture presentation">
<button type="button" data-architecture-view="flow" aria-pressed="true">Flow</button>
<button type="button" data-architecture-view="matrix" aria-pressed="false">Matrix</button>
<button type="button" data-architecture-view="selected" aria-pressed="false" disabled>Selected service</button>
<button type="button" id="architecture-edit-layout" hidden disabled>Edit layout</button>
</nav>
<p id="architecture-layout-notice" class="architecture-layout-notice" aria-live="polite" tabindex="-1" hidden></p>
<div class="canvas-tools">
<button id="zoom-out" title="Zoom out" aria-label="Zoom out">−</button>
<button id="zoom-in" title="Zoom in" aria-label="Zoom in">+</button>
<button id="reset-view" title="Reset zoom and pan" aria-label="Reset zoom and pan">100%</button>
<button id="fit-button" title="Fit visible graph" aria-label="Fit visible graph">Fit</button>
<button id="toggle-labels" title="Toggle labels" aria-label="Toggle relationship labels" aria-pressed="false">Labels</button>
<span id="zoom-readout" class="readout" aria-live="polite">100%</span>
</div>
<aside class="architecture-overlay-legend" aria-label="Architecture map legend">
<span title="Calls grouped across services"><i class="architecture-legend-line grouped"></i>Grouped calls</span>
<span title="Calls directly connected to the selected service"><i class="architecture-legend-line direct"></i>Direct calls</span>
<span title="A relationship requiring attention"><i class="architecture-legend-line risk"></i>Risk</span>
<span title="The currently selected service"><i class="architecture-legend-selection"></i>Selected</span>
</aside>
<section id="architecture-focus-panel" class="architecture-focus-panel" aria-label="Architecture focus">
<div class="architecture-domain-controls"><strong>Domains</strong><div id="architecture-domain-chips" class="architecture-domain-chips"></div></div>
<div class="architecture-direction-controls" role="group" aria-label="Relationship direction">
<button type="button" data-architecture-direction="outgoing" aria-pressed="false">Outgoing</button>
<button type="button" data-architecture-direction="incoming" aria-pressed="false">Incoming</button>
<button type="button" data-architecture-direction="both" aria-pressed="true">Both</button>
</div>
<button type="button" id="architecture-risk-toggle" aria-pressed="false">Risk</button>
<button type="button" id="reset-architecture-focus">Reset focus</button>
<div id="architecture-relationship-summary" class="architecture-relationship-summary" aria-live="polite" hidden>
<strong id="architecture-summary-service">No service selected</strong>
<span><b id="architecture-summary-incoming">0</b> incoming</span>
<span><b id="architecture-summary-outgoing">0</b> outgoing</span>
<span><b id="architecture-summary-resolution">0 resolved · 0 unresolved · 0 mismatch</b></span>
</div>
<div id="architecture-relationship-tooltip" class="architecture-relationship-tooltip" role="tooltip" hidden></div>
</section>
<section id="architecture-layout-editor" class="architecture-layout-editor" role="dialog" aria-modal="true" aria-labelledby="architecture-layout-title" data-layout-mode="manual" tabindex="-1" hidden>
<header class="architecture-layout-editor-header">
<div><h1 id="architecture-layout-title">Edit architecture layout</h1><p>Rename and reorder groups, or move services between groups. Changes are written only when you save.</p></div>
<div class="architecture-layout-editor-actions">
<button type="button" id="architecture-save-layout">Save</button>
<button type="button" id="architecture-discard-layout">Discard</button>
<button type="button" id="architecture-reset-layout">Reset to detected</button>
<button type="button" id="architecture-reload-layout" hidden>Reload latest and reapply draft</button>
<button type="button" id="architecture-close-layout">Close</button>
</div>
<p id="architecture-layout-status" class="architecture-layout-status" aria-live="polite">Ready.</p>
</header>
<div id="architecture-layout-groups" class="architecture-layout-groups"></div>
</section>
<svg id="workspace-graph" role="group" aria-label="Directed workspace relationship map"><defs><marker id="arrow" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#8fa2ae"></path></marker><marker id="arrow-focus" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#0b6b79"></path></marker><marker id="arrow-outgoing" markerWidth="13" markerHeight="11" refX="11" refY="5.5" orient="auto"><path d="M0,0 L13,5.5 L0,11 z" fill="#0b6b79"></path></marker><marker id="arrow-incoming" markerWidth="13" markerHeight="11" refX="11" refY="5.5" orient="auto"><path d="M0,0 L13,5.5 L0,11 z" fill="#a56a00"></path></marker></defs><g id="graph-layer"></g></svg>
<section id="workspace-workbench" class="workspace-workbench" hidden aria-label="Workspace data workbench"></section>
</main>
<section class="details" id="details"><p class="empty">Select a service or endpoint to inspect directed relationships.</p></section>
</div>`

func renderWorkspaceDashboardDocument(title string, payload []byte) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title>\n<style>")
	b.WriteString(workspaceDashboardStyles)
	b.WriteString("</style>\n</head>\n<body>\n")
	b.WriteString(workspaceDashboardShell)
	b.WriteString("\n<script>\nconst workspacePayload = ")
	b.Write(payload)
	b.WriteString(";\n")
	b.WriteString(workspaceDashboardArchitectureModelScript)
	b.WriteString("\n")
	b.WriteString(workspaceDashboardScript)
	b.WriteString("\n</script>\n</body>\n</html>")
	return b.String()
}

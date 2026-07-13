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
<button data-view-mode="endpoints" aria-pressed="false">Endpoints</button>
<button data-view-mode="data-flow" aria-pressed="false">Data Flow</button>
<button data-view-mode="diagnostics" aria-pressed="false">Diagnostics</button>
<button data-view-mode="coverage" aria-pressed="false">Coverage</button>
</div>
<p class="help" id="mode-help">See how projects and services communicate across the workspace. Select a service to highlight direct incoming and outgoing relationships without changing the layout.</p>
<input id="workspace-search" aria-label="Search workspace map" placeholder="Search service, endpoint, route, file, symbol">
<h2>Filter</h2>
<div class="filters">
<button data-kind-filter="all" class="active" aria-pressed="true">All</button>
<button data-kind-filter="risk" aria-pressed="false">Risk</button>
<button data-kind-filter="resolved" aria-pressed="false">Resolved</button>
<button data-kind-filter="unresolved" aria-pressed="false">Unresolved</button>
</div>
<div class="filters" style="margin-top:8px"><button id="clear-selection" type="button">Clear selection</button></div>
<div class="filters" style="margin-top:8px"><button id="isolate-neighborhood" type="button" hidden>Isolate neighborhood</button><button id="show-full-architecture" type="button" hidden>Show full architecture</button></div>
<div class="filters" style="margin-top:8px"><button id="focus-selected" type="button" hidden>Focus selected</button><button id="back-to-full-architecture" type="button" hidden>Back to full architecture</button></div>
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
<div class="canvas-tools">
<button id="zoom-out" title="Zoom out" aria-label="Zoom out">−</button>
<button id="zoom-in" title="Zoom in" aria-label="Zoom in">+</button>
<button id="reset-view" title="Reset zoom and pan" aria-label="Reset zoom and pan">100%</button>
<button id="fit-button" title="Fit visible graph" aria-label="Fit visible graph">Fit</button>
<button id="toggle-labels" title="Toggle labels" aria-label="Toggle relationship labels" aria-pressed="false">Labels</button>
<span id="zoom-readout" class="readout" aria-live="polite">100%</span>
</div>
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
	b.WriteString(workspaceDashboardScript)
	b.WriteString("\n</script>\n</body>\n</html>")
	return b.String()
}

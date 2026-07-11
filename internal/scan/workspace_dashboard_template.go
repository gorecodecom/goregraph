package scan

import (
	"html"
	"strings"
)

const workspaceDashboardShell = `<div class="shell">
<aside class="side">
<h1>GoreGraph Workspace</h1>
<div class="summary"><div class="metric"><strong id="service-count">0</strong><span>services</span></div><div class="metric"><strong id="edge-count">0</strong><span>relations</span></div><div class="metric"><strong id="trace-count">0</strong><span>traces</span></div></div>
<h2>View</h2>
<div class="modes">
<button data-view-mode="issues" class="active">Open Issues</button>
<button data-view-mode="architecture">Architecture Map</button>
<button data-view-mode="services">Focused Service</button>
<button data-view-mode="endpoint">Endpoint Trace</button>
<button data-view-mode="raw">Endpoint Paths</button>
</div>
<p class="help" id="mode-help">Open Issues groups unresolved, mismatched, and out-of-scope contracts by cause so duplicates can be triaged as one problem family.</p>
<input id="workspace-search" aria-label="Search workspace map" placeholder="Search service, endpoint, route, file, symbol">
<h2>Filter</h2>
<div class="filters">
<button data-kind-filter="all" class="active">All</button>
<button data-kind-filter="risk">Risk</button>
<button data-kind-filter="resolved">Resolved</button>
<button data-kind-filter="unresolved">Unresolved</button>
</div>
<div class="filters" style="margin-top:8px"><button id="clear-selection" type="button">Clear selection</button></div>
<h2 id="list-title">Services</h2>
<div id="node-list" class="item-list"></div>
<p id="result-note" class="result-note"></p>
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
<button id="zoom-out" title="Zoom out">-</button>
<button id="zoom-in" title="Zoom in">+</button>
<button id="reset-view" title="Reset zoom and pan">100%</button>
<button id="fit-button" title="Fit visible graph">Fit</button>
<button id="toggle-labels" title="Toggle labels">Labels</button>
<span id="zoom-readout" class="readout">100%</span>
</div>
<svg id="workspace-graph" role="img" aria-label="Directed workspace relationship map"><defs><marker id="arrow" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#8fa2ae"></path></marker><marker id="arrow-focus" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#0b6b79"></path></marker></defs><g id="graph-layer"></g></svg>
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

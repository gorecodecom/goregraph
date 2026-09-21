package scan

import (
	"embed"
	"html"
	"strings"

	"github.com/gorecodecom/goregraph/internal/version"
)

//go:embed dashboard/*.js dashboard/*.css dashboard/*.html
var dashboardFiles embed.FS

func dashboardFile(name string) string {
	body, err := dashboardFiles.ReadFile("dashboard/" + name)
	if err != nil {
		panic(err)
	}
	return string(body)
}

// renderWorkspaceDashboardDocument keeps the existing offline data contract and
// content-addressed usage shards. The extended tools also host the layout editor.
func renderWorkspaceDashboardDocument(title string, payload []byte) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"de\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><link rel=\"icon\" href=\"data:,\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title><style id=\"workspace-modern-styles\">")
	b.WriteString(dashboardFile("styles.css"))
	b.WriteString("</style><style id=\"workspace-extended-styles\" media=\"not all\">")
	b.WriteString(workspaceDashboardStyles)
	b.WriteString("</style></head><body>")
	b.WriteString(strings.ReplaceAll(dashboardFile("shell.html"), "__GOREGRAPH_VERSION__", html.EscapeString(version.Version)))
	b.WriteString("<template id=\"workspace-extended-shell\">")
	b.WriteString(workspaceDashboardShell)
	b.WriteString("</template><script type=\"text/plain\" id=\"workspace-extended-script\">")
	b.WriteString(workspaceDashboardArchitectureModelScript)
	b.WriteString("\n")
	b.WriteString(workspaceDashboardScript)
	b.WriteString("\n</script>\n<script>\nconst workspacePayload = ")
	b.Write(payload)
	b.WriteString(";\n(function(){\n")
	b.WriteString(`if(globalThis.__goregraphEditor?.editor_enabled||location.hash==='#advanced'){
      const shell=document.getElementById('workspace-extended-shell').content.cloneNode(true);
      const code=document.getElementById('workspace-extended-script').textContent;
      document.body.replaceChildren(shell);
      document.documentElement.lang='en';
      document.getElementById('workspace-modern-styles').disabled=true;
      document.getElementById('workspace-extended-styles').media='all';
      const script=document.createElement('script');script.textContent=code;document.body.appendChild(script);
      const back=document.createElement('a');back.textContent='Workspace Explorer';back.href='#architecture';back.onclick=event=>{event.preventDefault();location.hash='architecture';location.reload();};document.getElementById('workspace-sidebar').prepend(back);
      return;
    }
    document.getElementById('workspace-extended-shell').remove();
    document.getElementById('workspace-extended-script').remove();
`)
	for _, name := range []string{"adapter.js", "evidence-model.js", "app.js", "workspace-ui.js"} {
		b.WriteString(dashboardFile(name))
		b.WriteString("\n")
	}
	b.WriteString("})();\n</script>\n</body></html>")
	return b.String()
}

package handlers

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"jarvis-gateway/internal/config"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
	),
)

const pageTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - JARVIS</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link href="https://fonts.googleapis.com/css2?family=Orbitron:wght@400;700&family=Rajdhani:wght@400;500;600&family=JetBrains+Mono&display=swap" rel="stylesheet">
    <style>
        :root {
            --jarvis-blue: #00d4ff;
            --jarvis-blue-dim: #0099cc;
            --jarvis-orange: #ff6b35;
            --jarvis-bg: #0a0e14;
            --jarvis-surface: #0d1219;
            --jarvis-sidebar: #080b0f;
            --jarvis-border: #1a2332;
            --jarvis-text: #e0e6ed;
            --jarvis-text-dim: #6b7c93;
            --glow-blue: 0 0 20px rgba(0, 212, 255, 0.3);
            --glow-orange: 0 0 20px rgba(255, 107, 53, 0.3);
        }

        * { box-sizing: border-box; margin: 0; padding: 0; }

        body {
            font-family: 'Rajdhani', sans-serif;
            font-weight: 500;
            line-height: 1.7;
            background: var(--jarvis-bg);
            color: var(--jarvis-text);
            min-height: 100vh;
            display: flex;
        }

        /* Grid background */
        body::before {
            content: '';
            position: fixed;
            top: 0; left: 0; right: 0; bottom: 0;
            background-image:
                linear-gradient(rgba(0, 212, 255, 0.02) 1px, transparent 1px),
                linear-gradient(90deg, rgba(0, 212, 255, 0.02) 1px, transparent 1px);
            background-size: 50px 50px;
            pointer-events: none;
            z-index: -1;
        }

        @keyframes pulse {
            0%, 100% { opacity: 0.3; }
            50% { opacity: 0.8; }
        }

        @keyframes slideIn {
            from { opacity: 0; transform: translateX(-10px); }
            to { opacity: 1; transform: translateX(0); }
        }

        /* Sidebar */
        .sidebar {
            width: 280px;
            min-width: 280px;
            background: var(--jarvis-sidebar);
            border-right: 1px solid var(--jarvis-border);
            display: flex;
            flex-direction: column;
            position: fixed;
            top: 0;
            left: 0;
            height: 100vh;
            overflow: hidden;
            z-index: 100;
        }

        .sidebar-header {
            padding: 25px 20px;
            border-bottom: 1px solid var(--jarvis-border);
            position: relative;
        }

        .sidebar-header::after {
            content: '';
            position: absolute;
            bottom: 0;
            left: 20px;
            right: 20px;
            height: 1px;
            background: linear-gradient(90deg, var(--jarvis-blue), transparent);
        }

        .logo {
            font-family: 'Orbitron', monospace;
            font-size: 1.4em;
            font-weight: 700;
            color: var(--jarvis-blue);
            text-transform: uppercase;
            letter-spacing: 4px;
            text-shadow: var(--glow-blue);
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .logo-icon {
            width: 35px;
            height: 35px;
            border: 2px solid var(--jarvis-blue);
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            animation: pulse 3s ease-in-out infinite;
        }

        .logo-icon::before {
            content: '';
            width: 12px;
            height: 12px;
            background: var(--jarvis-blue);
            border-radius: 50%;
        }

        .sidebar-nav {
            flex: 1;
            overflow-y: auto;
            padding: 15px 0;
        }

        .nav-section {
            padding: 0 15px;
            margin-bottom: 20px;
        }

        .nav-section-title {
            font-family: 'Orbitron', monospace;
            font-size: 0.7em;
            color: var(--jarvis-text-dim);
            text-transform: uppercase;
            letter-spacing: 2px;
            padding: 10px 10px 8px;
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .nav-section-title::before {
            content: '';
            width: 4px;
            height: 4px;
            background: var(--jarvis-orange);
            border-radius: 50%;
        }

        .nav-list {
            list-style: none;
        }

        .nav-item {
            margin: 2px 0;
        }

        .nav-link {
            display: flex;
            align-items: center;
            gap: 10px;
            padding: 10px 15px;
            color: var(--jarvis-text-dim);
            text-decoration: none;
            font-size: 0.95em;
            border-radius: 6px;
            transition: all 0.2s ease;
            position: relative;
            border: 1px solid transparent;
        }

        .nav-link::before {
            content: '';
            width: 6px;
            height: 6px;
            border: 1px solid var(--jarvis-text-dim);
            border-radius: 2px;
            transition: all 0.2s ease;
        }

        .nav-link:hover {
            color: var(--jarvis-blue);
            background: rgba(0, 212, 255, 0.05);
            border-color: rgba(0, 212, 255, 0.2);
        }

        .nav-link:hover::before {
            border-color: var(--jarvis-blue);
            background: var(--jarvis-blue);
        }

        .nav-link.active {
            color: var(--jarvis-blue);
            background: rgba(0, 212, 255, 0.1);
            border-color: var(--jarvis-blue);
            box-shadow: var(--glow-blue);
        }

        .nav-link.active::before {
            border-color: var(--jarvis-blue);
            background: var(--jarvis-blue);
            box-shadow: 0 0 8px var(--jarvis-blue);
        }

        .health-btn {
            width: 100%;
            text-align: left;
            cursor: pointer;
            background: none;
            font-family: inherit;
            font-size: inherit;
        }

        .health-btn:hover {
            background: rgba(0, 212, 255, 0.05);
        }

        .health-details {
            margin-top: 10px;
            font-size: 0.75em;
            padding: 8px 10px;
            background: var(--jarvis-surface);
            border-radius: 6px;
            display: none;
        }

        .health-details.show {
            display: block;
        }

        .health-details .detail-row {
            display: flex;
            justify-content: space-between;
            padding: 3px 0;
            border-bottom: 1px solid var(--jarvis-border);
        }

        .health-details .detail-row:last-child {
            border-bottom: none;
        }

        .health-details .label {
            color: var(--jarvis-text-dim);
        }

        .health-details .value {
            color: var(--jarvis-blue);
        }

        .status-dot.checking {
            background: var(--jarvis-orange);
            animation: pulse 0.5s ease-in-out infinite;
        }

        .status-dot.error {
            background: #ff4444;
        }

        .sidebar-footer {
            padding: 15px 20px;
            border-top: 1px solid var(--jarvis-border);
            font-size: 0.8em;
            color: var(--jarvis-text-dim);
        }

        .status-indicator {
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .status-dot {
            width: 8px;
            height: 8px;
            background: #00ff88;
            border-radius: 50%;
            animation: pulse 2s ease-in-out infinite;
        }

        /* Main content */
        .main {
            flex: 1;
            margin-left: 280px;
            min-height: 100vh;
        }

        .topbar {
            position: sticky;
            top: 0;
            background: rgba(10, 14, 20, 0.95);
            backdrop-filter: blur(10px);
            border-bottom: 1px solid var(--jarvis-border);
            padding: 15px 30px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            z-index: 50;
        }

        .breadcrumb {
            display: flex;
            align-items: center;
            gap: 10px;
            font-size: 0.9em;
        }

        .breadcrumb a {
            color: var(--jarvis-text-dim);
            text-decoration: none;
            transition: color 0.2s;
        }

        .breadcrumb a:hover {
            color: var(--jarvis-blue);
        }

        .breadcrumb-sep {
            color: var(--jarvis-border);
        }

        .breadcrumb-current {
            color: var(--jarvis-blue);
            font-weight: 600;
        }

        .topbar-actions {
            display: flex;
            gap: 15px;
        }

        .topbar-btn {
            padding: 8px 16px;
            background: transparent;
            border: 1px solid var(--jarvis-border);
            color: var(--jarvis-text-dim);
            border-radius: 6px;
            font-family: inherit;
            font-size: 0.85em;
            cursor: pointer;
            transition: all 0.2s;
            text-decoration: none;
        }

        .topbar-btn:hover {
            border-color: var(--jarvis-blue);
            color: var(--jarvis-blue);
            box-shadow: var(--glow-blue);
        }

        .content {
            max-width: 900px;
            padding: 40px 50px;
            animation: slideIn 0.3s ease-out;
        }

        /* Typography */
        h1 {
            font-family: 'Orbitron', monospace;
            font-size: 2em;
            color: var(--jarvis-blue);
            margin-bottom: 30px;
            text-transform: uppercase;
            letter-spacing: 2px;
            position: relative;
            padding-bottom: 15px;
        }

        h1::after {
            content: '';
            position: absolute;
            bottom: 0; left: 0;
            width: 60px; height: 3px;
            background: var(--jarvis-orange);
            box-shadow: var(--glow-orange);
        }

        h2 {
            font-family: 'Orbitron', monospace;
            font-size: 1.3em;
            color: var(--jarvis-text);
            margin: 40px 0 20px;
            padding-bottom: 10px;
            border-bottom: 1px solid var(--jarvis-border);
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        h3 {
            font-size: 1.15em;
            color: var(--jarvis-blue-dim);
            margin: 25px 0 12px;
        }

        p { margin: 15px 0; }

        a {
            color: var(--jarvis-blue);
            text-decoration: none;
            transition: all 0.2s ease;
        }

        a:hover {
            color: var(--jarvis-orange);
            text-shadow: var(--glow-orange);
        }

        code {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.85em;
            background: var(--jarvis-surface);
            color: var(--jarvis-orange);
            padding: 3px 8px;
            border-radius: 4px;
            border: 1px solid var(--jarvis-border);
        }

        pre {
            background: var(--jarvis-surface);
            border: 1px solid var(--jarvis-border);
            border-left: 3px solid var(--jarvis-blue);
            border-radius: 6px;
            padding: 20px;
            margin: 20px 0;
            overflow-x: auto;
        }

        pre code {
            background: none;
            border: none;
            padding: 0;
            color: var(--jarvis-text);
        }

        table {
            width: 100%;
            border-collapse: collapse;
            margin: 25px 0;
            font-size: 0.9em;
        }

        th {
            font-family: 'Orbitron', monospace;
            background: var(--jarvis-surface);
            color: var(--jarvis-blue);
            text-transform: uppercase;
            letter-spacing: 1px;
            font-size: 0.8em;
            padding: 12px 15px;
            text-align: left;
            border-bottom: 2px solid var(--jarvis-blue);
        }

        td {
            padding: 10px 15px;
            border-bottom: 1px solid var(--jarvis-border);
        }

        tr:hover td {
            background: rgba(0, 212, 255, 0.03);
        }

        blockquote {
            border-left: 3px solid var(--jarvis-orange);
            background: rgba(255, 107, 53, 0.05);
            margin: 20px 0;
            padding: 15px 20px;
            border-radius: 0 6px 6px 0;
            color: var(--jarvis-text-dim);
        }

        ul, ol {
            margin: 15px 0;
            padding-left: 25px;
        }

        li { margin: 6px 0; }

        /* Index page cards */
        .doc-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
            gap: 15px;
            margin-top: 30px;
        }

        .doc-card {
            background: var(--jarvis-surface);
            border: 1px solid var(--jarvis-border);
            border-radius: 8px;
            padding: 20px;
            transition: all 0.3s ease;
            position: relative;
            overflow: hidden;
        }

        .doc-card::before {
            content: '';
            position: absolute;
            top: 0; left: 0;
            width: 100%; height: 3px;
            background: linear-gradient(90deg, var(--jarvis-blue), var(--jarvis-orange));
            transform: scaleX(0);
            transform-origin: left;
            transition: transform 0.3s ease;
        }

        .doc-card:hover {
            border-color: var(--jarvis-blue);
            box-shadow: var(--glow-blue);
            transform: translateY(-3px);
        }

        .doc-card:hover::before {
            transform: scaleX(1);
        }

        .doc-card a {
            font-size: 1.1em;
            font-weight: 600;
            display: block;
        }

        .doc-card-desc {
            font-size: 0.85em;
            color: var(--jarvis-text-dim);
            margin-top: 8px;
        }

        /* Mobile */
        .mobile-toggle {
            display: none;
            position: fixed;
            top: 15px;
            left: 15px;
            z-index: 200;
            width: 40px;
            height: 40px;
            background: var(--jarvis-surface);
            border: 1px solid var(--jarvis-border);
            border-radius: 8px;
            cursor: pointer;
            align-items: center;
            justify-content: center;
        }

        .mobile-toggle span {
            width: 20px;
            height: 2px;
            background: var(--jarvis-blue);
            display: block;
            position: relative;
        }

        .mobile-toggle span::before,
        .mobile-toggle span::after {
            content: '';
            position: absolute;
            width: 100%;
            height: 2px;
            background: var(--jarvis-blue);
            left: 0;
        }

        .mobile-toggle span::before { top: -6px; }
        .mobile-toggle span::after { bottom: -6px; }

        @media (max-width: 900px) {
            .mobile-toggle { display: flex; }
            .sidebar {
                transform: translateX(-100%);
                transition: transform 0.3s ease;
            }
            .sidebar.open { transform: translateX(0); }
            .main { margin-left: 0; }
            .content { padding: 30px 20px; }
            .topbar { padding: 15px 20px; padding-left: 70px; }
        }
    </style>
</head>
<body>
    <button class="mobile-toggle" onclick="document.querySelector('.sidebar').classList.toggle('open')">
        <span></span>
    </button>

    <aside class="sidebar">
        <div class="sidebar-header">
            <div class="logo">
                <div class="logo-icon"></div>
                JARVIS
            </div>
        </div>
        <nav class="sidebar-nav">
            <div class="nav-section">
                <div class="nav-section-title">Documentation</div>
                <ul class="nav-list">
                    {{range .Docs}}
                    <li class="nav-item">
                        <a href="/docs/{{.Name}}" class="nav-link{{if eq .Name $.Current}} active{{end}}">{{.Name}}</a>
                    </li>
                    {{end}}
                </ul>
            </div>
            <div class="nav-section">
                <div class="nav-section-title">System</div>
                <ul class="nav-list">
                    <li class="nav-item">
                        <button class="nav-link health-btn" onclick="checkHealth()">
                            Health Check
                        </button>
                    </li>
                </ul>
            </div>
        </nav>
        <div class="sidebar-footer">
            <div class="status-indicator" id="health-status">
                <span class="status-dot" id="status-dot"></span>
                <span id="status-text">System Online</span>
            </div>
            <div id="health-details" class="health-details"></div>
        </div>
    </aside>

    <main class="main">
        <header class="topbar">
            <div class="breadcrumb">
                <a href="/docs">Docs</a>
                {{if .Current}}
                <span class="breadcrumb-sep">/</span>
                <span class="breadcrumb-current">{{.Current}}</span>
                {{end}}
            </div>
            <div class="topbar-actions">
                {{if .Current}}
                <a href="/docs" class="topbar-btn">All Docs</a>
                {{end}}
            </div>
        </header>
        <article class="content">
            {{.Content}}
        </article>
    </main>

    <script>
        // Close sidebar on mobile when clicking a link
        document.querySelectorAll('.nav-link:not(.health-btn)').forEach(link => {
            link.addEventListener('click', () => {
                if (window.innerWidth <= 900) {
                    document.querySelector('.sidebar').classList.remove('open');
                }
            });
        });

        // Health check function
        async function checkHealth() {
            const dot = document.getElementById('status-dot');
            const text = document.getElementById('status-text');
            const details = document.getElementById('health-details');

            dot.className = 'status-dot checking';
            text.textContent = 'Checking...';
            details.classList.remove('show');

            try {
                const start = performance.now();
                const res = await fetch('/health');
                const latency = Math.round(performance.now() - start);
                const data = await res.json();

                if (res.ok && data.status === 'ok') {
                    dot.className = 'status-dot';
                    dot.style.background = '#00ff88';
                    text.textContent = 'Online';

                    details.innerHTML =
                        '<div class="detail-row"><span class="label">Status</span><span class="value">OK</span></div>' +
                        '<div class="detail-row"><span class="label">Latency</span><span class="value">' + latency + 'ms</span></div>' +
                        '<div class="detail-row"><span class="label">Time</span><span class="value">' + new Date().toLocaleTimeString() + '</span></div>';
                    details.classList.add('show');
                } else {
                    throw new Error('Unhealthy');
                }
            } catch (e) {
                dot.className = 'status-dot error';
                text.textContent = 'Error';
                details.innerHTML = '<div class="detail-row"><span class="label">Error</span><span class="value" style="color:#ff4444">' + e.message + '</span></div>';
                details.classList.add('show');
            }
        }

        // Auto-check on load
        checkHealth();
    </script>
</body>
</html>`

var tmpl = template.Must(template.New("page").Parse(pageTemplate))

type DocInfo struct {
	Name string
}

type PageData struct {
	Title   string
	Current string
	Docs    []DocInfo
	Content template.HTML
}

func Docs(cfg *config.Config) http.HandlerFunc {
	docsPath := cfg.DocsPath
	if docsPath == "" {
		docsPath = "/opt/obsidian-vault/Coding/OpenClaw"
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Get list of all docs for sidebar
		docs := getDocList(docsPath)

		// Extract doc name from path: /docs/Status -> Status
		path := strings.TrimPrefix(r.URL.Path, "/docs")
		path = strings.TrimPrefix(path, "/")

		if path == "" {
			// List all docs
			serveDocList(w, docs)
			return
		}

		// Serve specific doc
		serveDoc(w, docsPath, path, docs)
	}
}

func getDocList(docsPath string) []DocInfo {
	var docs []DocInfo

	filepath.WalkDir(docsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			name := strings.TrimSuffix(d.Name(), ".md")
			docs = append(docs, DocInfo{Name: name})
		}
		return nil
	})

	// Sort alphabetically
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})

	return docs
}

func serveDocList(w http.ResponseWriter, docs []DocInfo) {
	var buf bytes.Buffer
	buf.WriteString("<h1>Documentation</h1>\n")
	buf.WriteString("<p>JARVIS system documentation and configuration reference.</p>\n")
	buf.WriteString("<div class=\"doc-grid\">\n")

	descriptions := map[string]string{
		"Configuration":          "Full configuration reference",
		"Credentials":            "API tokens and access keys",
		"Implementation-Summary": "Implementation details",
		"Plugins":                "Available plugins",
		"README":                 "Project overview",
		"Roadmap":                "Development roadmap 2026",
		"Status":                 "Current system status",
		"Test-Results":           "Testing documentation",
		"TODO":                   "Current tasks",
	}

	for _, doc := range docs {
		desc := descriptions[doc.Name]
		if desc == "" {
			desc = "Documentation"
		}
		buf.WriteString("<div class=\"doc-card\">")
		buf.WriteString("<a href=\"/docs/" + doc.Name + "\">" + doc.Name + "</a>")
		buf.WriteString("<div class=\"doc-card-desc\">" + desc + "</div>")
		buf.WriteString("</div>\n")
	}
	buf.WriteString("</div>")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, PageData{
		Title:   "Documentation",
		Current: "",
		Docs:    docs,
		Content: template.HTML(buf.String()),
	})
}

func serveDoc(w http.ResponseWriter, docsPath, name string, docs []DocInfo) {
	// Security: prevent path traversal
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		http.Error(w, "Invalid document name", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(docsPath, name+".md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	var htmlBuf bytes.Buffer
	if err := md.Convert(content, &htmlBuf); err != nil {
		http.Error(w, "Failed to render document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, PageData{
		Title:   name,
		Current: name,
		Docs:    docs,
		Content: template.HTML(htmlBuf.String()),
	})
}

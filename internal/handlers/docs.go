package handlers

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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
            position: relative;
            overflow-x: hidden;
        }

        /* Grid background */
        body::before {
            content: '';
            position: fixed;
            top: 0; left: 0; right: 0; bottom: 0;
            background-image:
                linear-gradient(rgba(0, 212, 255, 0.03) 1px, transparent 1px),
                linear-gradient(90deg, rgba(0, 212, 255, 0.03) 1px, transparent 1px);
            background-size: 50px 50px;
            pointer-events: none;
            z-index: -1;
        }

        /* Animated corner accents */
        body::after {
            content: '';
            position: fixed;
            top: 20px; left: 20px;
            width: 100px; height: 100px;
            border-left: 2px solid var(--jarvis-blue);
            border-top: 2px solid var(--jarvis-blue);
            opacity: 0.5;
            animation: pulse 3s ease-in-out infinite;
        }

        @keyframes pulse {
            0%, 100% { opacity: 0.3; }
            50% { opacity: 0.7; }
        }

        @keyframes slideIn {
            from { opacity: 0; transform: translateY(20px); }
            to { opacity: 1; transform: translateY(0); }
        }

        @keyframes glow {
            0%, 100% { box-shadow: var(--glow-blue); }
            50% { box-shadow: 0 0 30px rgba(0, 212, 255, 0.5); }
        }

        .container {
            max-width: 1000px;
            margin: 0 auto;
            padding: 40px 30px;
            animation: slideIn 0.5s ease-out;
        }

        /* Navigation */
        .nav {
            display: flex;
            align-items: center;
            gap: 30px;
            padding: 20px 0;
            margin-bottom: 40px;
            border-bottom: 1px solid var(--jarvis-border);
            position: relative;
        }

        .nav::after {
            content: '';
            position: absolute;
            bottom: -1px;
            left: 0;
            width: 150px;
            height: 2px;
            background: linear-gradient(90deg, var(--jarvis-blue), transparent);
        }

        .nav-logo {
            font-family: 'Orbitron', monospace;
            font-size: 1.5em;
            font-weight: 700;
            color: var(--jarvis-blue);
            text-transform: uppercase;
            letter-spacing: 3px;
            text-shadow: var(--glow-blue);
        }

        .nav a {
            color: var(--jarvis-text-dim);
            text-decoration: none;
            font-size: 0.95em;
            text-transform: uppercase;
            letter-spacing: 1px;
            padding: 8px 16px;
            border: 1px solid transparent;
            border-radius: 4px;
            transition: all 0.3s ease;
            position: relative;
        }

        .nav a:hover {
            color: var(--jarvis-blue);
            border-color: var(--jarvis-blue);
            background: rgba(0, 212, 255, 0.1);
            box-shadow: var(--glow-blue);
        }

        /* Headings */
        h1 {
            font-family: 'Orbitron', monospace;
            font-size: 2.2em;
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
            font-size: 1.4em;
            color: var(--jarvis-text);
            margin: 40px 0 20px;
            padding-bottom: 10px;
            border-bottom: 1px solid var(--jarvis-border);
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        h3 {
            font-size: 1.2em;
            color: var(--jarvis-blue-dim);
            margin: 30px 0 15px;
        }

        /* Links */
        a {
            color: var(--jarvis-blue);
            text-decoration: none;
            transition: all 0.2s ease;
        }

        a:hover {
            color: var(--jarvis-orange);
            text-shadow: var(--glow-orange);
        }

        /* Code */
        code {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.9em;
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
            position: relative;
        }

        pre::before {
            content: 'CODE';
            position: absolute;
            top: -10px; left: 15px;
            font-family: 'Orbitron', monospace;
            font-size: 0.7em;
            color: var(--jarvis-blue);
            background: var(--jarvis-bg);
            padding: 2px 8px;
            letter-spacing: 2px;
        }

        pre code {
            background: none;
            border: none;
            padding: 0;
            color: var(--jarvis-text);
        }

        /* Tables */
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 25px 0;
            font-size: 0.95em;
        }

        th {
            font-family: 'Orbitron', monospace;
            background: var(--jarvis-surface);
            color: var(--jarvis-blue);
            text-transform: uppercase;
            letter-spacing: 1px;
            font-size: 0.85em;
            padding: 15px;
            text-align: left;
            border-bottom: 2px solid var(--jarvis-blue);
        }

        td {
            padding: 12px 15px;
            border-bottom: 1px solid var(--jarvis-border);
            transition: background 0.2s ease;
        }

        tr:hover td {
            background: rgba(0, 212, 255, 0.05);
        }

        /* Document list */
        .doc-list {
            list-style: none;
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
            gap: 15px;
            margin-top: 30px;
        }

        .doc-list li {
            background: var(--jarvis-surface);
            border: 1px solid var(--jarvis-border);
            border-radius: 8px;
            transition: all 0.3s ease;
            position: relative;
            overflow: hidden;
        }

        .doc-list li::before {
            content: '';
            position: absolute;
            top: 0; left: 0;
            width: 3px; height: 100%;
            background: var(--jarvis-blue);
            transform: scaleY(0);
            transition: transform 0.3s ease;
        }

        .doc-list li:hover {
            border-color: var(--jarvis-blue);
            box-shadow: var(--glow-blue);
            transform: translateX(5px);
        }

        .doc-list li:hover::before {
            transform: scaleY(1);
        }

        .doc-list a {
            display: block;
            padding: 20px;
            font-size: 1.1em;
            font-weight: 600;
        }

        /* Blockquote */
        blockquote {
            border-left: 3px solid var(--jarvis-orange);
            background: rgba(255, 107, 53, 0.05);
            margin: 20px 0;
            padding: 15px 20px;
            border-radius: 0 6px 6px 0;
            color: var(--jarvis-text-dim);
        }

        /* Lists */
        ul, ol {
            margin: 15px 0;
            padding-left: 25px;
        }

        li {
            margin: 8px 0;
        }

        /* Status indicator */
        .status {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            padding: 5px 12px;
            background: rgba(0, 212, 255, 0.1);
            border: 1px solid var(--jarvis-blue);
            border-radius: 20px;
            font-size: 0.85em;
            color: var(--jarvis-blue);
        }

        .status::before {
            content: '';
            width: 8px; height: 8px;
            background: var(--jarvis-blue);
            border-radius: 50%;
            animation: pulse 2s ease-in-out infinite;
        }

        /* Footer accent */
        .container::after {
            content: '';
            display: block;
            margin-top: 60px;
            height: 1px;
            background: linear-gradient(90deg, var(--jarvis-blue), transparent 50%);
        }

        /* Responsive */
        @media (max-width: 768px) {
            .container { padding: 20px 15px; }
            h1 { font-size: 1.6em; }
            .nav { flex-wrap: wrap; gap: 15px; }
            .doc-list { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <nav class="nav">
            <span class="nav-logo">JARVIS</span>
            <a href="/docs">Documentation</a>
            <a href="/health">System Status</a>
        </nav>
        <main>{{.Content}}</main>
    </div>
</body>
</html>`

var tmpl = template.Must(template.New("page").Parse(pageTemplate))

type PageData struct {
	Title   string
	Content template.HTML
}

func Docs(cfg *config.Config) http.HandlerFunc {
	docsPath := cfg.DocsPath
	if docsPath == "" {
		docsPath = "/opt/obsidian-vault/Coding/OpenClaw"
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Extract doc name from path: /docs/Status -> Status
		path := strings.TrimPrefix(r.URL.Path, "/docs")
		path = strings.TrimPrefix(path, "/")

		if path == "" {
			// List all docs
			serveDocList(w, docsPath)
			return
		}

		// Serve specific doc
		serveDoc(w, docsPath, path)
	}
}

func serveDocList(w http.ResponseWriter, docsPath string) {
	var docs []string

	filepath.WalkDir(docsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			name := strings.TrimSuffix(d.Name(), ".md")
			docs = append(docs, name)
		}
		return nil
	})

	var buf bytes.Buffer
	buf.WriteString("<h1>📚 JARVIS Documentation</h1>\n<ul class=\"doc-list\">\n")
	for _, doc := range docs {
		buf.WriteString("<li><a href=\"/docs/" + doc + "\">" + doc + "</a></li>\n")
	}
	buf.WriteString("</ul>")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, PageData{
		Title:   "Documentation",
		Content: template.HTML(buf.String()),
	})
}

func serveDoc(w http.ResponseWriter, docsPath, name string) {
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
		Content: template.HTML(htmlBuf.String()),
	})
}

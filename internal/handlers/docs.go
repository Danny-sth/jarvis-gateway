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
    <title>{{.Title}} - JARVIS Docs</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            line-height: 1.6;
            max-width: 900px;
            margin: 0 auto;
            padding: 20px;
            background: #0d1117;
            color: #c9d1d9;
        }
        a { color: #58a6ff; text-decoration: none; }
        a:hover { text-decoration: underline; }
        h1, h2, h3 { color: #f0f6fc; border-bottom: 1px solid #21262d; padding-bottom: 0.3em; }
        code {
            background: #161b22;
            padding: 0.2em 0.4em;
            border-radius: 3px;
            font-family: 'Fira Code', monospace;
        }
        pre {
            background: #161b22;
            padding: 16px;
            border-radius: 6px;
            overflow-x: auto;
        }
        pre code { padding: 0; background: none; }
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 16px 0;
        }
        th, td {
            border: 1px solid #30363d;
            padding: 8px 12px;
            text-align: left;
        }
        th { background: #161b22; }
        .nav {
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 1px solid #21262d;
        }
        .nav a { margin-right: 15px; }
        .doc-list { list-style: none; padding: 0; }
        .doc-list li { padding: 8px 0; border-bottom: 1px solid #21262d; }
        .doc-list a { font-size: 1.1em; }
        blockquote {
            border-left: 4px solid #3fb950;
            margin: 0;
            padding-left: 16px;
            color: #8b949e;
        }
    </style>
</head>
<body>
    <div class="nav">
        <a href="/docs">📚 Docs</a>
        <a href="/health">💚 Health</a>
    </div>
    <main>{{.Content}}</main>
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

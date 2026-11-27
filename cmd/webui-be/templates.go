package main

import (
	"fmt"
	"html/template"
)

func parseTemplates(debug bool) (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatBytes": formatBytes,
		"or": func(a, b string) string {
			if a != "" {
				return a
			}
			return b
		},
		"list": func(args ...string) []string { return args },
	}
	files := []string{
		"web/templates/base.html",
		"web/templates/dashboard.html",
		"web/templates/torrents.html",
		"web/templates/disk.html",
		"web/templates/filebot_form.html",
		"web/templates/filebot_result.html",
		"web/templates/filebot_not_found.html",
	}
	if debug {
		files = append(files, "web/templates/playground.html")
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(webFS, files...)
	if err != nil {
		return nil, fmt.Errorf("parseTemplates: %w", err)
	}
	return tmpl, nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

package stats

import (
	"path"
	"strings"
)

var extLanguage = map[string]string{
	".go": "Go", ".py": "Python", ".js": "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript",
	".ts": "TypeScript", ".tsx": "TypeScript", ".rs": "Rust", ".java": "Java", ".kt": "Kotlin",
	".c": "C", ".h": "C", ".cc": "C++", ".cpp": "C++", ".hpp": "C++", ".cs": "C#",
	".rb": "Ruby", ".php": "PHP", ".swift": "Swift", ".m": "Objective-C", ".scala": "Scala",
	".sh": "Shell", ".bash": "Shell", ".zsh": "Shell", ".ps1": "PowerShell",
	".html": "HTML", ".css": "CSS", ".scss": "SCSS", ".vue": "Vue", ".svelte": "Svelte",
	".md": "Markdown", ".json": "JSON", ".yaml": "YAML", ".yml": "YAML", ".toml": "TOML",
	".xml": "XML", ".sql": "SQL", ".lua": "Lua", ".ex": "Elixir", ".exs": "Elixir",
	".hs": "Haskell", ".dart": "Dart", ".r": "R", ".proto": "Protobuf", ".tf": "Terraform",
	".txt": "Text",
}

var nameLanguage = map[string]string{
	"Makefile": "Makefile", "Dockerfile": "Dockerfile", "CMakeLists.txt": "CMake",
}

// languageOf maps a path to a language by file name, then extension.
func languageOf(p string) string {
	base := path.Base(p)
	if l, ok := nameLanguage[base]; ok {
		return l
	}
	if l, ok := extLanguage[strings.ToLower(path.Ext(base))]; ok {
		return l
	}
	return "Other"
}

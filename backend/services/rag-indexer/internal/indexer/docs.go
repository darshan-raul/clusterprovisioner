package indexer

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type DocChunk struct {
	ID       string
	Path     string
	Section  string
	Text     string
	Metadata map[string]any
}

// LoadAndChunkDocs traverses a directory of markdown files and splits them into sections.
func LoadAndChunkDocs(docsDir string) ([]DocChunk, error) {
	var chunks []DocChunk

	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		return chunks, nil
	}

	err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		relPath, _ := filepath.Rel(docsDir, path)
		fileChunks, chunkErr := chunkMarkdownFile(path, relPath)
		if chunkErr == nil {
			chunks = append(chunks, fileChunks...)
		}
		return nil
	})

	return chunks, err
}

func chunkMarkdownFile(filePath, relPath string) ([]DocChunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunks []DocChunk
	scanner := bufio.NewScanner(file)

	currentSection := "Overview"
	var currentLines []string

	flush := func() {
		text := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if text != "" {
			h := sha256.Sum256([]byte(fmt.Sprintf("%s#%s#%s", relPath, currentSection, text)))
			chunkID := fmt.Sprintf("doc-%x", h[:8])
			chunks = append(chunks, DocChunk{
				ID:      chunkID,
				Path:    relPath,
				Section: currentSection,
				Text:    fmt.Sprintf("Document: %s | Section: %s\n%s", relPath, currentSection, text),
				Metadata: map[string]any{
					"path":    relPath,
					"section": currentSection,
					"kind":    "doc",
				},
			})
		}
		currentLines = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			flush()
			currentSection = strings.TrimLeft(trimmed, "# ")
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	return chunks, scanner.Err()
}

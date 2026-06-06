package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/unidoc/unioffice/document"
)

// FileParser handles parsing of different file formats.
type FileParser struct{}

// NewFileParser creates a new FileParser instance.
func NewFileParser() *FileParser {
	return &FileParser{}
}

// ParseFile parses the file content based on its MIME type and returns the text.
func (p *FileParser) ParseFile(data []byte, filename string) (string, error) {
	ext := strings.ToLower(filename[strings.LastIndex(filename, "."):])

	switch ext {
	case ".pdf":
		return p.ParsePDF(data)
	case ".txt":
		return p.ParseTXT(data)
	case ".docx":
		return p.ParseDOCX(data)
	default:
		return "", fmt.Errorf("unsupported file type: %s (expected .pdf, .txt, or .docx)", ext)
	}
}

// ParsePDF extracts text content from a PDF file.
func (p *FileParser) ParsePDF(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	pdfDoc, err := pdf.FromReader(reader)
	if err != nil {
		return "", fmt.Errorf("failed to parse PDF: %w", err)
	}
	defer pdfDoc.Close()

	var textParts []string
	pageCount := pdfDoc.PageCount()

	for i := 0; i < pageCount; i++ {
		page := pdfDoc.Page(i)
		if page.V == nil {
			continue
		}

		text, err := page.Text()
		if err != nil {
			continue
		}
		textParts = append(textParts, text)
	}

	return strings.Join(textParts, "\n"), nil
}

// ParseTXT reads plain text content directly.
func (p *FileParser) ParseTXT(data []byte) (string, error) {
	return string(data), nil
}

// ParseDOCX extracts text content from a DOCX file.
func (p *FileParser) ParseDOCX(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	doc, err := document.Open(reader)
	if err != nil {
		return "", fmt.Errorf("failed to parse DOCX: %w", err)
	}

	var textParts []string
	for _, section := range doc.Sections() {
		for _, element := range section.Elements() {
			switch e := element.(type) {
			case document.Paragraph:
				for _, run := range e.Runs() {
					text := run.Text()
					if text != "" {
						textParts = append(textParts, text)
					}
				}
			}
		}
	}

	return strings.Join(textParts, "\n"), nil
}
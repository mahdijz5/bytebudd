package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// unused import marker - operators is intentionally unused
var _ = []string{}

// FileParser handles parsing of different file formats.
type FileParser struct{}

// NewFileParser creates a new FileParser instance.
func NewFileParser() *FileParser {
	return &FileParser{}
}

// ParseFile parses the file content based on its extension and returns the text.
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

// ParsePDF extracts text content from a PDF file using a simple text extraction approach.
func (p *FileParser) ParsePDF(data []byte) (string, error) {
	_ = bytes.NewReader(data) // Simple PDF text extraction - look for text streams
	var textParts []string
	
	// Try to find text content in PDF streams
	content := string(data)
	
	// Look for text between BT (begin text) and ET (end text) operators
	startIdx := 0
	for {
		btIdx := strings.Index(content[startIdx:], "BT")
		if btIdx == -1 {
			break
		}
		
		etIdx := strings.Index(content[startIdx+btIdx:], "ET")
		if etIdx == -1 {
			break
		}
		
		textBlock := content[startIdx+btIdx+2 : startIdx+btIdx+etIdx]
		// Extract text from Tj and TJ operators
		textParts = append(textParts, extractTextFromPDFBlock(textBlock))
		
		startIdx = startIdx + btIdx + etIdx + 2
	}
	
	if len(textParts) > 0 {
		return strings.Join(textParts, "\n"), nil
	}
	
	// Fallback: try to extract readable text
	textParts = extractReadableText(content)
	return strings.Join(textParts, "\n"), nil
}

// extractTextFromPDFBlock extracts readable text from a PDF text block.
func extractTextFromPDFBlock(block string) string {
	var texts []string
	
	// Look for (text) Tj pattern
	remaining := block
	for {
		tjIdx := strings.Index(remaining, "Tj")
		if tjIdx == -1 {
			break
		}
		
		// Find the closing parenthesis before Tj
		searchStart := tjIdx - 1
		escaped := false
		parenIdx := -1
		
		for i := searchStart; i >= 0; i-- {
			if escaped {
				escaped = false
				continue
			}
			if remaining[i] == '\\' {
				escaped = true
				continue
			}
			if remaining[i] == ')' {
				parenIdx = i
				break
			}
		}
		
		if parenIdx != -1 {
			text := remaining[parenIdx+1 : tjIdx]
			texts = append(texts, decodePDFString(text))
		}
		
		remaining = remaining[tjIdx+2:]
	}
	
	return strings.Join(texts, " ")
}

// decodePDFString decodes a PDF string literal, handling escape sequences.
func decodePDFString(s string) string {
	var result strings.Builder
	escaped := false
	
	for _, ch := range s {
		if escaped {
			switch ch {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			case '(':
				result.WriteByte('(')
			case ')':
				result.WriteByte(')')
			case '\\':
				result.WriteByte('\\')
			default:
				result.WriteRune(ch)
			}
			escaped = false
		} else if ch == '\\' {
			escaped = true
		} else {
			result.WriteRune(ch)
		}
	}
	
	return result.String()
}

// extractReadableText extracts readable ASCII text from PDF content.
func extractReadableText(content string) []string {
	var texts []string
	
	// Simple approach: extract sequences of printable characters
	var current strings.Builder
	for _, ch := range content {
		if (ch >= 32 && ch <= 126) || ch == '\n' || ch == '\r' || ch == '\t' {
			current.WriteRune(ch)
		} else if current.Len() > 0 {
			text := strings.TrimSpace(current.String())
			if len(text) > 3 {
				texts = append(texts, text)
			}
			current.Reset()
		}
	}
	
	if current.Len() > 0 {
		text := strings.TrimSpace(current.String())
		if len(text) > 3 {
			texts = append(texts, text)
		}
	}
	
	return texts
}

// ParseTXT reads plain text content directly.
func (p *FileParser) ParseTXT(data []byte) (string, error) {
	return string(data), nil
}

// ParseDOCX extracts text content from a DOCX file by unzipping and reading the XML.
func (p *FileParser) ParseDOCX(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	
	// Open the zip archive
	zipReader, err := zip.NewReader(reader, reader.Size())
	if err != nil {
		return "", fmt.Errorf("failed to parse DOCX as ZIP: %w", err)
	}
	
	// Find and read word/document.xml
	var docXML []byte
	for _, file := range zipReader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			defer rc.Close()
			
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			docXML = buf.Bytes()
			break
		}
	}
	
	if len(docXML) == 0 {
		return "", fmt.Errorf("document.xml not found in DOCX")
	}
	
	// Parse XML and extract text
	var textParts []string
	
	// Simple XML text extraction
	doc := &DOCXDocument{}
	if err := xml.Unmarshal(docXML, doc); err != nil {
		return "", fmt.Errorf("failed to parse DOCX XML: %w", err)
	}
	
	for _, para := range doc.Paragraphs {
		if para.Text != "" {
			textParts = append(textParts, para.Text)
		}
	}
	
	return strings.Join(textParts, "\n"), nil
}

// DOCXDocument represents the structure of word/document.xml
type DOCXDocument struct {
	XMLName      xml.Name `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main document"`
	Paragraphs   []Paragraph
}

// Paragraph represents a paragraph in a DOCX document
type Paragraph struct {
	Text string `xml:"*>t"`
}
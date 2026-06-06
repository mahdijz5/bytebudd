package main

import (
	"strings"
	"unicode"
)

// Chunker handles splitting text into overlapping chunks.
type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
}

// NewChunker creates a new Chunker with the given size and overlap.
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
	}
}

// Chunk splits text into overlapping chunks, splitting on word boundaries.
func (c *Chunker) Chunk(text string) []string {
	if text == "" {
		return nil
	}

	// Convert to runes to handle Unicode properly
	runes := []rune(text)
	length := len(runes)

	if length <= c.ChunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < length {
		end := start + c.ChunkSize
		if end > length {
			end = length
		}

		// Try to split on space boundary
		chunkEnd := end
		if end < length {
			// Look backwards for a space to avoid cutting words
			searchRange := c.ChunkOverlap + 10
			if searchRange > end {
				searchRange = end
			}

			for i := end - 1; i >= end-searchRange; i-- {
				if i >= 0 && runes[i] == ' ' {
					chunkEnd = i
					break
				}
			}

			// If no space found, look forward from end
			if chunkEnd == end && end < length {
				for i := end; i < length && i < end+50; i++ {
					if runes[i] == ' ' || runes[i] == '\n' || runes[i] == '\t' {
						chunkEnd = i
						break
					}
				}
			}
		}

		chunk := string(runes[start:chunkEnd])
		// Trim whitespace
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		// Check if we're done
		if chunkEnd >= length {
			break
		}

		// Move start forward with overlap
		start = chunkEnd - c.ChunkOverlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

// ChunkWithSource wraps chunks with their source filename.
func (c *Chunker) ChunkWithSource(text, source string) []DocumentChunk {
	chunks := c.Chunk(text)
	var result []DocumentChunk

	for _, chunk := range chunks {
		result = append(result, DocumentChunk{
			PageContent: chunk,
			Source:      source,
		})
	}

	return result
}

// WordCount returns the number of words in a string.
func WordCount(s string) int {
	return len(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r)
	}))
}
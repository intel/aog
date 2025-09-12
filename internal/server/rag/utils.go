package rag

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider/template"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
)

var RagPromptTemplate = `
你是一个专业的知识助手，请根据以下提供的上下文来回答用户的问题。  

上下文（可能不完整或包含多个片段）：  
%s

用户问题：  
%s

要求：  
1. 如果上下文中有答案，请基于上下文尽可能准确回答。  
2. 如果上下文没有明确答案，请说明“我在提供的资料中没有找到相关信息”，然后根据自己的认知进行回答。  
3. 回答要简洁、结构清晰。  
`

type ChunkScore struct {
	ChunkID    string
	Content    string
	Similarity float32
}

var RagConfig, _ = getRagConfig()

func GetFileType(fileExt string) string {
	fileExt = strings.ToLower(fileExt)
	switch fileExt {
	case ".txt", ".md", ".html":
		return "text"
	case ".pdf":
		return "pdf"
	case ".docx":
		return "word"
	case ".xlsx":
		return "excel"
	default:
		return "other"
	}
}

func getRagConfig() (*types.RagServiceConfig, error) {
	data, err := template.FlavorTemplateFs.ReadFile("rag_config.json")
	if err != nil {
		return nil, bcode.WrapError(bcode.ErrReadRequestBody, err)
	}
	var ragConfig types.RagServiceConfig
	err = json.Unmarshal(data, &ragConfig)
	if err != nil {
		return nil, bcode.WrapError(bcode.ErrReadRequestBody, err)
	}
	return &ragConfig, nil
}

func ChunkFile(filePath string, chunkSize int) ([]string, error) {
	fileExt := strings.ToLower(filepath.Ext(filePath))
	// Extract text from a binary file first
	switch fileExt {
	case ".pdf":
		// Extract text from a binary file first
		text, err := utils.ExtractTextFromPDF(filePath)
		if err != nil || len(text) == 0 {
			return nil, fmt.Errorf("PDF内容提取失败或为空：%w", err)
		}
		return utils.ChunkTextContent(text, chunkSize), nil
	case ".docx":
		text, err := utils.ExtractTextFromDocx(filePath)
		if err != nil || len(text) == 0 {
			return nil, fmt.Errorf("Word内容提取失败或为空：%w", err)
		}
		return utils.ChunkTextContent(text, chunkSize), nil
	case ".xlsx":
		text, err := utils.ExtractTextFromXlsx(filePath)
		if err != nil || len(text) == 0 {
			return nil, fmt.Errorf("Excel内容提取失败或为空：%w", err)
		}
		return utils.ChunkTextContent(text, chunkSize), nil
	}
	// Other formats follow the original logic
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	const minChunkSize = 100
	const defaultChunkSize = 1024
	const maxChunkSize = 8192 // 8KB upper limit to prevent large blocks

	if chunkSize < minChunkSize {
		logger.LogicLogger.Warn("块大小过小，调整为默认值", "original_size", chunkSize, "new_size", defaultChunkSize)
		chunkSize = defaultChunkSize
	} else if chunkSize > maxChunkSize {
		logger.LogicLogger.Warn("块大小过大，调整为上限", "original_size", chunkSize, "new_size", maxChunkSize)
		chunkSize = maxChunkSize
	}
	// Selecting a chunking strategy based on file type
	fileExt = strings.ToLower(filepath.Ext(filePath))
	switch fileExt {
	case ".md", ".txt", ".log", ".rst", ".asciidoc":
		return chunkTextFileByParagraphs(file, chunkSize)

	case ".json", ".xml", ".yaml", ".yml", ".toml":
		logger.LogicLogger.Debug("使用专用分块器处理结构化文件", "ext", fileExt)
		file.Close()
		return ChunkStructuredFile(filePath, chunkSize)

	case ".csv", ".tsv":
		logger.LogicLogger.Debug("处理表格文件", "ext", fileExt)
		return chunkTextFileByLines(file, chunkSize)

	default:
		return chunkTextFileByLines(file, chunkSize)
	}
}

// Block by row to keep rows intact
func chunkTextFileByLines(file *os.File, chunkSize int) ([]string, error) {
	var chunks []string
	scanner := bufio.NewScanner(file)

	// Set up a larger buffer to handle long delays.
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	currentChunk := ""
	for scanner.Scan() {
		line := scanner.Text()

		// If the current line plus the previous content exceeds the block size, and the current block is not empty,
		// Save the current block and start a new one.
		if len(currentChunk)+len(line)+1 > chunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = line
		} else {
			if len(currentChunk) > 0 {
				currentChunk += "\n"
			}
			currentChunk += line
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	// Check for scan errors
	if err := scanner.Err(); err != nil {
		return chunks, fmt.Errorf("扫描文件时出错: %w", err)
	}

	logger.LogicLogger.Debug("文件已分块", "path", file.Name(), "chunks", len(chunks))
	return chunks, nil
}

// Divide by paragraph and try to keep the paragraph intact
func chunkTextFileByParagraphs(file *os.File, chunkSize int) ([]string, error) {
	var chunks []string
	scanner := bufio.NewScanner(file)

	// Set buffer
	const maxScanTokenSize = 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	currentChunk := ""
	currentParagraph := ""
	emptyLineCount := 0

	// Process each line
	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 {
			// An empty line may indicate the end of a paragraph
			emptyLineCount++
			if len(currentParagraph) > 0 {
				currentParagraph += "\n"
			}
		} else {
			// Non-empty line, add to the current paragraph
			if emptyLineCount > 1 && len(currentParagraph) > 0 {
				// Multiple empty lines may indicate separation between paragraphs
				if len(currentChunk) > 0 && len(currentChunk)+len(currentParagraph)+2 > chunkSize {
					// Current paragraph plus the current chunk exceeds the size; save the current chunk
					chunks = append(chunks, currentChunk)
					currentChunk = currentParagraph
				} else {
					// Add the paragraph to the current chunk
					if len(currentChunk) > 0 {
						currentChunk += "\n\n"
					}
					currentChunk += currentParagraph
				}
				currentParagraph = line
			} else {
				// Continue the current paragraph
				if len(currentParagraph) > 0 {
					currentParagraph += "\n"
				}
				currentParagraph += line
			}
			emptyLineCount = 0
		} // Handle paragraph
		if len(currentParagraph) > chunkSize {
			// If the paragraph itself exceeds chunkSize, force split
			if len(currentChunk) > 0 {
				// Save the current chunk first
				chunks = append(chunks, currentChunk)
				currentChunk = ""
			}

			// Split the large paragraph into smaller pieces
			paragraphRunes := []rune(currentParagraph)
			for i := 0; i < len(paragraphRunes); i += chunkSize {
				end := i + chunkSize
				if end > len(paragraphRunes) {
					end = len(paragraphRunes)
				}
				// Try to split at sentence boundaries
				sentenceEnd := findSentenceBoundary(paragraphRunes, i, end)
				if sentenceEnd > i {
					end = sentenceEnd
				}
				chunks = append(chunks, string(paragraphRunes[i:end]))
				if end >= len(paragraphRunes) {
					break
				}
			}
			currentParagraph = ""
		} else if len(currentParagraph) > chunkSize/2 {
			// If the current paragraph exceeds half the chunk size, add it to the chunk
			if len(currentChunk) > 0 && len(currentChunk)+len(currentParagraph)+2 > chunkSize {
				// Save the current chunk
				chunks = append(chunks, currentChunk)
				currentChunk = currentParagraph
			} else {
				// Add to the current chunk
				if len(currentChunk) > 0 {
					currentChunk += "\n\n"
				}
				currentChunk += currentParagraph
			}
			currentParagraph = ""
		}

		// If the current chunk exceeds the chunk size, save it
		if len(currentChunk) > chunkSize {
			chunks = append(chunks, currentChunk)
			currentChunk = ""
		}
	}

	// Handle the remaining content
	if len(currentParagraph) > 0 {
		if len(currentChunk) > 0 && len(currentChunk)+len(currentParagraph)+2 > chunkSize {
			chunks = append(chunks, currentChunk)
			chunks = append(chunks, currentParagraph)
		} else {
			if len(currentChunk) > 0 {
				currentChunk += "\n\n"
			}
			currentChunk += currentParagraph
			chunks = append(chunks, currentChunk)
		}
	} else if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	// Check for scanning errors
	if err := scanner.Err(); err != nil {
		return chunks, fmt.Errorf("扫描文件时出错: %w", err)
	}

	logger.LogicLogger.Debug("文件已按段落分块", "path", file.Name(), "chunks", len(chunks))
	return chunks, nil
}

// ChunkStructuredFile
func ChunkStructuredFile(filePath string, maxChunkSize int) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		return chunkJSONFile(filePath, maxChunkSize)
	case ".xml":
		return chunkXMLFile(filePath, maxChunkSize)
	case ".yaml", ".yml":
		return chunkYAMLFile(filePath, maxChunkSize)
	default:
		// Unsupported structured formats, chunked by line
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader := bufio.NewReader(file)
		return ChunkReaderByLines(reader, maxChunkSize)
	}
}

// ChunkReaderByLines Block by row
func ChunkReaderByLines(reader io.Reader, maxChunkSize int) ([]string, error) {
	var chunks []string
	scanner := bufio.NewScanner(reader)
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)
	currentChunk := ""
	for scanner.Scan() {
		line := scanner.Text()
		if len(currentChunk)+len(line)+1 > maxChunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = line
		} else {
			if len(currentChunk) > 0 {
				currentChunk += "\n"
			}
			currentChunk += line
		}
	}
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}
	if err := scanner.Err(); err != nil {
		return chunks, err
	}
	return chunks, nil
}

// Block according to JSON structure, and block according to array elements or object top-level keys first
func chunkJSONFile(filePath string, maxChunkSize int) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var jsonObj interface{}
	err = json.Unmarshal(data, &jsonObj)
	if err != nil {
		slog.Warn("JSON解析失败，回退为按行分块", "file", filePath, "error", err)
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader := bufio.NewReader(file)
		return ChunkReaderByLines(reader, maxChunkSize)
	}

	switch obj := jsonObj.(type) {
	case []interface{}:
		return chunkJSONArray(obj, maxChunkSize)
	case map[string]interface{}:
		return chunkJSONObject(obj, maxChunkSize)
	default:
		return []string{string(data)}, nil
	}
}

// Blocking by JSON array elements
func chunkJSONArray(arr []interface{}, maxChunkSize int) ([]string, error) {
	var chunks []string
	var currentChunk strings.Builder
	currentChunk.WriteString("[\n")
	for _, item := range arr {
		itemBytes, err := json.MarshalIndent(item, "  ", "  ")
		if err != nil {
			slog.Warn("JSON数组元素序列化失败", "error", err)
			continue
		}
		itemStr := string(itemBytes)
		if currentChunk.Len()+len(itemStr)+4 > maxChunkSize && currentChunk.Len() > 2 {
			currentChunk.WriteString("\n]")
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			// 添加XML声明
			currentChunk.WriteString("[\n")
			currentChunk.WriteString("  " + itemStr)
		} else {
			if currentChunk.Len() > 2 {
				currentChunk.WriteString(",\n")
			}
			currentChunk.WriteString("  " + itemStr)
		}
	}
	if currentChunk.Len() > 2 {
		currentChunk.WriteString("\n]")
		chunks = append(chunks, currentChunk.String())
	}
	return chunks, nil
}

// Block by JSON object top-level key
func chunkJSONObject(obj map[string]interface{}, maxChunkSize int) ([]string, error) {
	var chunks []string
	var currentChunk strings.Builder
	currentChunk.WriteString("{\n")
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	for _, key := range keys {
		value := obj[key]
		valueBytes, err := json.MarshalIndent(value, "  ", "  ")
		if err != nil {
			slog.Warn("JSON对象值序列化失败", "key", key, "error", err)
			continue
		}
		keyValueStr := `  "` + key + `": ` + string(valueBytes)
		if currentChunk.Len()+len(keyValueStr)+4 > maxChunkSize && currentChunk.Len() > 2 {
			currentChunk.WriteString("\n}")
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentChunk.WriteString("{\n")
			currentChunk.WriteString(keyValueStr)
		} else {
			if currentChunk.Len() > 2 {
				currentChunk.WriteString(",\n")
			}
			currentChunk.WriteString(keyValueStr)
		}
	}
	if currentChunk.Len() > 2 {
		currentChunk.WriteString("\n}")
		chunks = append(chunks, currentChunk.String())
	}
	return chunks, nil
}

// Block according to the XML structure to keep the tags as intact as possible
func chunkXMLFile(filePath string, maxChunkSize int) ([]string, error) {
	// Read the entire file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))

	var chunks []string
	var currentChunk strings.Builder
	var depth int
	currentChunk.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("XML解析错误，回退为按行分块", "error", err)
			file, ferr := os.Open(filePath)
			if ferr != nil {
				return nil, ferr
			}
			defer file.Close()
			reader := bufio.NewReader(file)
			return ChunkReaderByLines(reader, maxChunkSize)
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && currentChunk.Len() > maxChunkSize/2 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
				// Add XML declaration
				currentChunk.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
			}
			currentChunk.WriteString("<" + t.Name.Local)
			for _, attr := range t.Attr {
				currentChunk.WriteString(` ` + attr.Name.Local + `="` + attr.Value + `"`)
			}
			currentChunk.WriteString(">")
		case xml.EndElement:
			currentChunk.WriteString("</" + t.Name.Local + ">")
			depth--
		case xml.CharData:
			currentChunk.WriteString(string(t))
		case xml.Comment:
			currentChunk.WriteString("<!--" + string(t) + "-->")
		}
		if currentChunk.Len() > maxChunkSize && depth <= 1 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			// Add XML declaration
			currentChunk.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		}
	}
	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}
	return chunks, nil
}

func chunkYAMLFile(filePath string, maxChunkSize int) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var chunks []string
	var currentChunk strings.Builder
	var inDocument bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
			}
			inDocument = true
			currentChunk.WriteString(line + "\n")
			continue
		}

		if strings.TrimSpace(line) == "..." {
			currentChunk.WriteString(line + "\n")
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			inDocument = false
			continue
		}

		if !strings.HasPrefix(line, " ") && strings.Contains(line, ":") {
			if currentChunk.Len()+len(line)+1 > maxChunkSize && currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
				if inDocument {
					currentChunk.WriteString("---\n")
				}
			}
		}

		currentChunk.WriteString(line + "\n")
	}
	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}
	if err := scanner.Err(); err != nil {
		return chunks, err
	}
	return chunks, nil
}

func findSentenceBoundary(text []rune, start, end int) int {
	// end-of-sentence marker
	sentenceEnders := []rune{'.', '!', '?', '。', '！', '？', '；', ';', '\n', '\r'}

	// Look backwards and forwards, trying to split at sentence boundaries
	for i := end - 1; i > start; i-- {
		// If you find the sentence end marker
		for _, ender := range sentenceEnders {
			if text[i] == ender {
				// Move back one bit, including the end tag
				if i+1 < end {
					return i + 1
				}
				return i
			}
		}

		// Find the paragraph separation
		if i+1 < end && text[i] == '\n' && text[i+1] == '\n' {
			return i
		}
	}

	// If the sentence boundary is not found, try splitting at the word boundary
	for i := end - 1; i > start+10; i-- {
		if unicode.IsSpace(text[i]) && !unicode.IsSpace(text[i+1]) {
			return i + 1
		}
	}

	return end
}

// CreateOverlappingChunks create overlapping blocks from the given text block
// chunks: original text block
// overlapSize: the size of the overlap (number of characters)
// Return a new block with overlap
func CreateOverlappingChunks(chunks []string, overlapSize int) []string {
	if overlapSize <= 0 || len(chunks) <= 1 {
		return chunks
	}

	result := make([]string, 0, len(chunks))

	result = append(result, chunks[0])

	for i := 1; i < len(chunks); i++ {
		prevChunk := chunks[i-1]
		currentChunk := chunks[i]

		overlapStart := 0
		if len(prevChunk) > overlapSize {
			overlapStart = len(prevChunk) - overlapSize
		}

		overlap := ""
		if overlapStart < len(prevChunk) {
			overlap = prevChunk[overlapStart:]
		}

		// 结合重叠部分和当前块
		newChunk := overlap
		if len(newChunk) > 0 && len(currentChunk) > 0 {
			newChunk += "\n\n... 继续前文 ...\n\n"
		}
		newChunk += currentChunk

		result = append(result, newChunk)
	}

	return result
}

func ApplyChunkOverlap(chunks []string, overlapSize int) []string {
	if overlapSize <= 0 || len(chunks) <= 1 {
		return chunks
	}
	return CreateOverlappingChunks(chunks, overlapSize)
}

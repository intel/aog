package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/datastore/sqlite"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/server/rag"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
)

type RagService interface {
	GetFile(ctx context.Context, request *dto.RagGetFileRequest) (*dto.RagGetFileResponse, error)
	GetFiles(ctx context.Context) (*dto.RagGetFilesResponse, error)
	UploadFile(c *gin.Context) (*dto.RagUploadFileResponse, error)
	DeleteFile(ctx context.Context, request *dto.RagDeleteFileRequest) (*dto.RagDeleteFileResponse, error)
	Retrieval(ctx context.Context, fileRecord *dto.RagRetrievalRequest) (*dto.RagRetrievalResponse, error)
}

type RagServiceImpl struct {
	Ds  datastore.Datastore
	JDs datastore.JsonDatastore
}

func NewRagService() *RagServiceImpl {
	return &RagServiceImpl{
		Ds:  datastore.GetDefaultDatastore(),
		JDs: datastore.GetDefaultJsonDatastore(),
	}
}

func (srv *RagServiceImpl) GetFile(ctx context.Context, request *dto.RagGetFileRequest) (*dto.RagGetFileResponse, error) {
	if request == nil || request.FileId == "" {
		return nil, fmt.Errorf("file_id is required")
	}
	entity := &types.RagFile{FileID: request.FileId}
	if err := srv.Ds.Get(ctx, entity); err != nil {
		return nil, err
	}
	return &dto.RagGetFileResponse{
		Bcode: *bcode.RagSuccessCode,
		Data:  *entity,
	}, nil
}

func (srv *RagServiceImpl) GetFiles(ctx context.Context) (*dto.RagGetFilesResponse, error) {
	list, err := srv.Ds.List(ctx, &types.RagFile{}, nil)
	if err != nil {
		return nil, err
	}
	files := make([]types.RagFile, 0, len(list))
	for _, e := range list {
		if rf, ok := e.(*types.RagFile); ok {
			files = append(files, *rf)
		}
	}
	return &dto.RagGetFilesResponse{
		Bcode: *bcode.RagSuccessCode,
		Data:  files,
	}, nil
}

func (srv *RagServiceImpl) UploadFile(c *gin.Context) (*dto.RagUploadFileResponse, error) {
	// check embed service
	s := new(types.Service)
	s.Name = types.ServiceEmbed
	if err := srv.Ds.Get(c.Request.Context(), s); err != nil {
		return nil, err
	}
	f, err := c.FormFile("file")
	if err != nil {
		logger.LogicLogger.Error("[RAG] Upload file error: cannot read request file")
		return nil, bcode.ErrRagBadRequest
	}
	dataDir, _ := utils.GetAOGDataDir()
	fileId := uuid.New().String()
	ext := filepath.Ext(f.Filename)
	if f.Size > types.RagServiceFileSize {
		return nil, bcode.ErrRagFileSize
	}
	if !utils.Contains(types.SupportRagServiceFileType, ext) {
		return nil, bcode.ErrRagFileType
	}
	dst := filepath.Join(dataDir, "ragFile", fileId, f.Filename)
	if err := c.SaveUploadedFile(f, dst); err != nil {
		logger.LogicLogger.Error("failed to save upload file: " + err.Error())
		return nil, bcode.ErrRagServerError
	}
	fileType := rag.GetFileType(ext)
	rf := &types.RagFile{
		FileID:    fileId,
		FileName:  f.Filename,
		FileType:  fileType,
		FilePath:  dst,
		Status:    1, // processing
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := srv.Ds.Add(c.Request.Context(), rf); err != nil {
		return nil, err
	}
	go ProcessFile(rf)
	data := &dto.RagUploadFileResponseData{
		FileId: fileId,
	}
	return &dto.RagUploadFileResponse{
		Bcode: *bcode.RagSuccessCode,
		Data:  *data,
	}, nil
}

func (srv *RagServiceImpl) DeleteFile(ctx context.Context, request *dto.RagDeleteFileRequest) (*dto.RagDeleteFileResponse, error) {
	if request == nil || request.FileId == "" {
		return nil, fmt.Errorf("file_id is required")
	}
	// cascade delete vectors and chunks (by index)
	_ = srv.Ds.Delete(ctx, &types.RagChunk{FileID: request.FileId})
	// delete file record
	if err := srv.Ds.Delete(ctx, &types.RagFile{FileID: request.FileId}); err != nil {
		return nil, err
	}
	return &dto.RagDeleteFileResponse{}, nil
}

func ProcessFile(fileRecord *types.RagFile) error {
	ctx := context.Background()
	ds := datastore.GetDefaultDatastore()
	chunkSize := rag.RagConfig.ChunkSize
	chunkOverlap := rag.RagConfig.ChunkOverlap
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	step := chunkSize - chunkOverlap
	if step <= 0 {
		step = chunkSize // 防止死循环
	}

	if fileRecord.Status != 1 { // 仅处理 processing
		return fmt.Errorf("invalid file status: %d", fileRecord.Status)
	}

	chunks, chunkErr := rag.ChunkFile(fileRecord.FilePath, rag.RagConfig.ChunkSize)
	if chunkErr != nil {
		logger.LogicLogger.Error("Failed to chunk file", "error", chunkErr)
		return chunkErr
	}
	// Apply overlap processing, the default overlap size is 10% of the block size.
	overlapSize := rag.RagConfig.ChunkSize / rag.RagConfig.ChunkOverlap
	if overlapSize > 200 {
		overlapSize = 200
	}
	if len(chunks) > 1 && overlapSize > 0 {
		logger.LogicLogger.Debug("应用块重叠处理", "chunk_count", len(chunks), "overlap_size", overlapSize)
		chunks = rag.ApplyChunkOverlap(chunks, overlapSize)
	}
	batchSize := 100
	RagChunks := make([]*types.RagChunk, 0, len(chunks))
	modelEngine := rag.NewEngineService()
	logger.LogicLogger.Info("Server: 开始批量生成embedding", "fileID", fileRecord.ID, "totalChunks", len(chunks), "batchSize", batchSize, "embedModel")

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		logger.LogicLogger.Info("Server: 处理embedding批次", "fileID", fileRecord.ID, "batchIndex", i/batchSize, "batchSize", len(batch))

		headers := http.Header{}
		headers.Set("Content-Type", "application/json")
		embedReq := &rag.EmbeddingRequest{
			Model: rag.RagConfig.EmbedModel,
			Input: batch,
		}

		embeddingResp, err := modelEngine.GenerateEmbedding(ctx, embedReq)
		if err != nil {
			logger.LogicLogger.Error("Failed to generate batch embeddings", "error", err, "fileID", fileRecord.FileID, "batchIndex", i/batchSize)
			return err
		}
		if len(embeddingResp.Embeddings) != len(batch) {
			logger.LogicLogger.Warn("嵌入数量与块数量不符", "embeddings", len(embeddingResp.Embeddings), "chunks", len(batch), "fileID", fileRecord.ID)
		}
		logger.LogicLogger.Info("Server: embedding批次生成完成", "fileID", fileRecord.ID, "batchIndex", i/batchSize, "embeddingCount", len(embeddingResp.Embeddings))
		for j, content := range batch {
			if j >= len(embeddingResp.Embeddings) {
				break
			}
			RagChunk := &types.RagChunk{
				ID:         uuid.New().String(),
				FileID:     fileRecord.FileID,
				Content:    content,
				ChunkIndex: i + j,
				Embedding:  embeddingResp.Embeddings[j],
				CreatedAt:  time.Now(),
			}
			RagChunks = append(RagChunks, RagChunk)
		}
	}

	for _, RagChunk := range RagChunks {
		err := ds.Put(ctx, RagChunk)
		if err != nil {
			logger.LogicLogger.Error("Failed to save file chunk", "error", err, "chunkID", RagChunk.ID, "fileID", RagChunk.FileID)
			return err
		}
	}
	fileRecord.Status = 2
	fileRecord.EmbedModel = rag.RagConfig.EmbedModel
	if err := ds.Put(ctx, fileRecord); err != nil {
		logger.LogicLogger.Error("Failed to save file record", "error", err, "fileID", fileRecord.FileID)
		return err
	}
	logger.LogicLogger.Info("Server: 文件embedding全部处理完成", "fileID", fileRecord.FileID, "totalChunks", len(RagChunks))
	return nil
}

// RAG Search: rag_config-based TopK and ScoreThreshold, similarity recall from aog_rag_chunk
func (srv *RagServiceImpl) Search(ctx context.Context, fileIDList []string, query string) (string, error) {
	logger.LogicLogger.Info("[RAG] findRelevantContextWithVec called", "query", query)

	// Check the initialization status of the VEC database
	if !sqlite.VecInitialized || sqlite.VecDB == nil {
		logger.LogicLogger.Warn("[RAG] VEC未初始化，尝试初始化")
		dbPath := config.GlobalEnvironment.Datastore
		if err := sqlite.InitVecDB(dbPath); err != nil {
			logger.LogicLogger.Error("[RAG] VEC初始化失败", "error", err)
			return "", fmt.Errorf("VEC初始化失败: %w", err)
		}

		// Check the initialization status again
		if !sqlite.VecInitialized || sqlite.VecDB == nil {
			logger.LogicLogger.Error("[RAG] VEC初始化后仍无效")
			return "", fmt.Errorf("VEC未正确初始化")
		}
	}
	modelEngine := rag.NewEngineService()

	// query expansion
	var queries []string
	queries = []string{query}
	// Generate embeddings for each query variant.
	var queryEmbeddings [][]float32
	for _, q := range queries {
		logger.LogicLogger.Info("[RAG] Generating embedding for query", "query", q, "embedModel", rag.RagConfig.EmbedModel)
		// Generate embeddings directly without using caching
		embeddingReq := &rag.EmbeddingRequest{
			Model: rag.RagConfig.EmbedModel,
			Input: []string{q},
		}
		embeddingResp, err := modelEngine.GenerateEmbedding(ctx, embeddingReq)
		if err != nil {
			logger.LogicLogger.Error("[RAG] 查询变体嵌入生成失败", "query", q, "error", err)
			return "", fmt.Errorf("RAG: 查询变体嵌入生成失败: %w", err)
		}
		if len(embeddingResp.Embeddings) == 0 {
			logger.LogicLogger.Error("[RAG] 嵌入返回数据为空", "query", q)
			return "", fmt.Errorf("RAG: 嵌入返回数据为空")
		}
		embedding := embeddingResp.Embeddings[0]
		logger.LogicLogger.Info("[RAG] Got embedding", "query", q, "embeddingDim", len(embedding))
		queryEmbeddings = append(queryEmbeddings, embedding)
	}
	logger.LogicLogger.Info("[RAG] 查询embedding生成完成", "successCount", len(queryEmbeddings), "totalQueries", len(queries))

	logger.LogicLogger.Info("DEBUG: queryEmbeddings after loop", "len", len(queryEmbeddings), "cap", cap(queryEmbeddings), "isNil", queryEmbeddings == nil)
	if len(queryEmbeddings) == 0 {
		logger.LogicLogger.Error("[RAG] 所有查询embedding生成失败")
		return "", fmt.Errorf("failed to generate query embeddings")
	} // Construct a file query to ensure that the SessionID has no spaces
	fileQuery := &types.RagFile{}
	logger.LogicLogger.Info("[RAG] 文件查询参数", "sessionID", fileQuery.FileID)

	queryOpts := &datastore.ListOptions{
		FilterOptions: datastore.FilterOptions{
			In: []datastore.InQueryOption{
				{
					Key:    "file_id",
					Values: fileIDList,
				},
			},
		},
	}

	files, err := srv.Ds.List(ctx, fileQuery, queryOpts)
	if err != nil {
		logger.LogicLogger.Error("RAG: 无法获取文件列表", "error", err)
		return "", err
	}

	if len(files) == 0 {
		logger.LogicLogger.Warn("RAG: 未找到关联文件，尝试其他查询方式")
	}

	if len(files) == 0 {
		logger.LogicLogger.Info("RAG: 没有找到关联文件")

		return "", nil
	}
	logger.LogicLogger.Info("RAG: 找到关联文件", "fileCount", len(files))

	if !sqlite.VecInitialized || sqlite.VecDB == nil {
		logger.LogicLogger.Error("[RAG] VEC数据库无法使用，尝试重新初始化")
		dbPath := config.GlobalEnvironment.Datastore
		if err := sqlite.InitVecDB(dbPath); err != nil || !sqlite.VecInitialized || sqlite.VecDB == nil {
			logger.LogicLogger.Error("[RAG] VEC初始化失败，无法执行检索", "error", err)
			return "", fmt.Errorf("VEC数据库不可用: %w", err)
		}
	}

	// Search similar blocks of all query variants using VEC (aggregate all results, deduplicate, sort)
	allChunks := make([]rag.ChunkScore, 0)
	chunkMap := make(map[string]rag.ChunkScore)
	startTime := time.Now()
	logger.LogicLogger.Info("RAG: 开始VEC检索", "embedCount", len(queryEmbeddings), "maxChunks", rag.RagConfig.TopK)
	for i, embedding := range queryEmbeddings { // Try vector search
		ids, dists, err := sqlite.VecDB.SearchSimilarChunks(ctx, embedding, rag.RagConfig.TopK, fileIDList)
		if err != nil {
			// Vector search failed, tried using text search as an alternative
			logger.LogicLogger.Warn("RAG: VEC搜索失败，尝试使用文本搜索", "queryIndex", i, "error", err) // 使用原始查询文本进行文本搜索
			textQuery := queries[i]
			ids, dists, err = sqlite.VecDB.SearchSimilarChunksByText(ctx, textQuery, rag.RagConfig.TopK, fileIDList)
			if err != nil {
				// Both search methods failed
				logger.LogicLogger.Error("RAG: 所有搜索方法均失败", "queryIndex", i, "error", err)
				return "", fmt.Errorf("RAG: 检索失败: %w", err)
			}
			logger.LogicLogger.Info("RAG: 降级使用文本搜索成功", "queryIndex", i)
		}
		logger.LogicLogger.Debug("RAG: 检索到相似块", "queryIndex", i, "resultCount", len(ids))
		for i, id := range ids {
			if _, exists := chunkMap[id]; exists {
				continue
			}
			chunkContent := ""
			chunkID := ""

			// Use only the primary key id to query the chunk content to avoid file_id interference
			if sqlite.VecDB != nil {
				chunkContent, chunkID, err = sqlite.VecDB.GetChunkByID(ctx, id)
				if err == nil && chunkContent != "" {
					logger.LogicLogger.Debug("RAG: 成功检索到chunk", "uuid", id, "chunkID", chunkID)
				} else {
					logger.LogicLogger.Warn("RAG: 获取文档块失败", "error", err, "uuid", id)
					continue
				}
			}

			chunkMap[id] = rag.ChunkScore{
				ChunkID:    chunkID,
				Content:    chunkContent,
				Similarity: float32(1.0 - dists[i]),
			}
		}
	}

	searchDuration := time.Since(startTime)

	logger.LogicLogger.Info("RAG: VEC批量搜索完成",
		"query_count", len(queryEmbeddings),
		"result_count", len(chunkMap),
		"duration_ms", searchDuration.Milliseconds())

	// Convert map to slice for sorting
	for _, chunk := range chunkMap {
		allChunks = append(allChunks, chunk)
	}

	// sort by similarity
	sort.Slice(allChunks, func(i, j int) bool {
		return allChunks[i].Similarity > allChunks[j].Similarity
	})

	if len(allChunks) > 3 {
		allChunks = allChunks[:3]
	}

	maxChunks := rag.RagConfig.TopK
	similarityThreshold := rag.RagConfig.ScoreThreshold

	if len(allChunks) < maxChunks {
		maxChunks = len(allChunks)
	}

	var relevantContext strings.Builder
	includedChunks := make(map[string]bool)
	var includedChunksCount int
	isDuplicate := func(newChunk rag.ChunkScore, existingChunks []rag.ChunkScore) bool {
		for _, existing := range existingChunks {
			if existing.ChunkID != newChunk.ChunkID &&
				(strings.Contains(existing.Content, newChunk.Content) ||
					strings.Contains(newChunk.Content, existing.Content)) {
				return true
			}
		}
		return false
	}
	var selectedChunks []rag.ChunkScore
	for i := 0; i < len(allChunks) && includedChunksCount < maxChunks; i++ {
		if allChunks[i].Similarity < 0.3 {
			logger.LogicLogger.Debug("块相似度低于阈值，已跳过",
				"chunk_id", allChunks[i].ChunkID,
				"similarity", allChunks[i].Similarity,
				"threshold", similarityThreshold)
			continue
		}
		if includedChunks[allChunks[i].ChunkID] {
			continue
		}
		if rag.RagConfig.DuplicationThreshold > 0 && isDuplicate(allChunks[i], selectedChunks) {
			logger.LogicLogger.Debug("跳过重复内容", "chunk_id", allChunks[i].ChunkID)
			continue
		}
		if relevantContext.Len() > 0 {
			relevantContext.WriteString("\n\n---\n\n")
		}
		relevantContext.WriteString("信息块#" + fmt.Sprint(includedChunksCount+1) +
			" (相似度: " + fmt.Sprintf("%.2f", allChunks[i].Similarity) + "):\n")
		relevantContext.WriteString(allChunks[i].Content)
		includedChunks[allChunks[i].ChunkID] = true
		selectedChunks = append(selectedChunks, allChunks[i])
		includedChunksCount++
	}
	logger.LogicLogger.Info("RAG: VEC-RAG检索完成",
		"query", query,
		"query_variants", len(queryEmbeddings),
		"total_chunks", len(allChunks),
		"included_chunks", includedChunksCount,
		"context_length", relevantContext.Len(),
		"similarity_threshold", rag.RagConfig.ScoreThreshold)

	return relevantContext.String(), nil
}

func (srv *RagServiceImpl) Retrieval(ctx context.Context, req *dto.RagRetrievalRequest) (*dto.RagRetrievalResponse, error) {
	service := &types.Service{}
	queryOpts := &datastore.ListOptions{
		FilterOptions: datastore.FilterOptions{
			In: []datastore.InQueryOption{
				{
					Key:    "name",
					Values: []string{types.ServiceEmbed, types.ServiceGenerate},
				},
			},
		},
	}
	services, err := srv.Ds.List(ctx, service, queryOpts)
	if err != nil {
		return nil, err
	}
	for _, s := range services {
		sObj := s.(*types.Service)
		if sObj.Name == types.ServiceEmbed && sObj.Status != 1 {
			return nil, fmt.Errorf("embed service is not avalivable")
		}
		if sObj.Name == types.ServiceGenerate && sObj.Status != 1 {
			return nil, fmt.Errorf("generate service is not avalivable")
		}
	}
	searchResult, err := srv.Search(ctx, req.FileIDs, req.Text)
	if err != nil {
		searchResult = "未查到相关文本"
	}

	prompt := fmt.Sprintf(rag.RagPromptTemplate, searchResult, req.Text)
	modelEngine := rag.NewEngineService()
	generateReq := &rag.GenerateRequest{
		Model:  req.Model,
		Prompt: prompt,
		Stream: false,
	}
	result, err := modelEngine.Generate(ctx, generateReq)
	if err != nil {
		return nil, err
	}
	RagResponseData := &dto.RagRetrievalResponseData{
		Model:   req.Model,
		Content: result.Content,
	}
	return &dto.RagRetrievalResponse{
		Bcode: *bcode.RagSuccessCode,
		Data:  *RagResponseData,
	}, nil
}

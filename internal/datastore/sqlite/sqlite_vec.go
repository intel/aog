package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/intel/aog/internal/logger"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

type VectorDBVec struct {
	db                     *sql.DB
	dimension              int
	availableDistanceFuncs []string
}

// VEC相关全局变量
var (
	VecDB          *VectorDBVec
	VecInitialized bool
	VecMutex       sync.Mutex
)

// 首先调用 Auto() 在全局初始化时注册所有函数
func init() {
	sqlite_vec.Auto()
}

func NewVectorDBVec(dbPath string) (*VectorDBVec, error) {
	// 打开数据库连接，确保允许加载扩展
	connStr := fmt.Sprintf("%s?_load_extension=1", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("打开sqlite数据库失败: %w", err)
	}

	// 检查是否启用了扩展加载
	var enableLoad int
	if err := db.QueryRow("PRAGMA enable_load_extension").Scan(&enableLoad); err != nil {
		fmt.Printf("警告：检查扩展加载状态失败: %v\n", err)
	} else if enableLoad != 1 {
		fmt.Println("警告：SQLite扩展加载功能未启用，尝试启用...")
		if _, err := db.Exec("PRAGMA enable_load_extension = 1"); err != nil {
			fmt.Printf("启用扩展加载失败: %v\n", err)
		}
	}

	// 创建向量数据库实例
	vdb := &VectorDBVec{
		db:                     db,
		dimension:              0,          // 默认维度，后续会检测
		availableDistanceFuncs: []string{}, // 默认没有可用函数，后续会检测
	}

	// 测试
	rows, err := vdb.db.Query("PRAGMA compile_options;")
	if err != nil {
		fmt.Println("查询 compile_options 出错:", err)
	} else {
		fmt.Println("==== SQLite compile_options ====")
		for rows.Next() {
			var opt string
			rows.Scan(&opt)
			fmt.Println("  ", opt)
		}
		rows.Close()
	}

	// 检测向量函数可用性
	vdb.detectVectorFunctions()

	// 输出诊断信息
	vdb.printDiagnosticInfo()

	return vdb, nil
}

// Close 关闭数据库连接并释放资源
func (vdb *VectorDBVec) Close() error {
	fmt.Println("关闭向量数据库连接")
	if vdb.db != nil {
		return vdb.db.Close()
	}
	return nil
}

// Initialize 初始化向量数据库结构
func (vdb *VectorDBVec) Initialize() error {
	fmt.Println("\n=== 初始化向量数据库结构 ===")

	// 检测向量维度
	if vdb.dimension <= 0 {
		// 尝试从现有数据中检测维度
		var embeddingStr string
		err := vdb.db.QueryRow("SELECT embedding FROM file_chunks WHERE embedding IS NOT NULL AND json_valid(embedding) = 1 LIMIT 1").Scan(&embeddingStr)
		if err == nil && embeddingStr != "" {
			// 成功获取为JSON格式
			var vecFromJson []float32
			if err := json.Unmarshal([]byte(embeddingStr), &vecFromJson); err == nil && len(vecFromJson) > 0 {
				fmt.Printf("从数据中检测到向量维度: %d\n", len(vecFromJson))
				vdb.dimension = len(vecFromJson)
			}
		}
	}

	return nil
}

// detectVectorFunctions 检测可用的向量函数
func (vdb *VectorDBVec) detectVectorFunctions() {
	fmt.Println("\n=== 开始检测向量函数 ===")

	// 1. 检查SQLite版本和扩展状态
	var sqliteVersion string
	if err := vdb.db.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion); err == nil {
		fmt.Printf("SQLite 版本: %s\n", sqliteVersion)
	} else {
		fmt.Printf("无法获取 SQLite 版本: %v\n", err)
	}

	// 检查扩展加载状态
	var enableLoad int
	if err := vdb.db.QueryRow("PRAGMA enable_load_extension").Scan(&enableLoad); err == nil {
		fmt.Printf("扩展加载状态: %d (1=已启用, 0=未启用)\n", enableLoad)
	}

	// 2. 检查是否已加载了 sqlite-vec 相关函数
	fmt.Println("检查向量函数可用性...")
	distanceFunctions := []string{
		"vec_distance_cosine",
		"vec_distance_L2",
		"vec_dot",
		"vec_serialize",
	}

	for _, funcName := range distanceFunctions {
		// 用合法空向量测试，避免类型错误
		sql := fmt.Sprintf("SELECT %s('[]', '[]')", funcName)
		_, err := vdb.db.Exec(sql)
		if err != nil {
			// 只要不是 "no such function" 就认为函数已注册
			if strings.Contains(err.Error(), "no such function") {
				continue
			}
		}
		vdb.availableDistanceFuncs = append(vdb.availableDistanceFuncs, funcName)
	}

	// 检测向量维度
	vdb.detectVectorDimensions()
}

// detectVectorDimensions 尝试从数据库中检测向量维度
func (vdb *VectorDBVec) detectVectorDimensions() {
	// 尝试确定向量维度并检查存储格式
	var embeddingStr string
	// 首先检查是否有JSON格式的embedding
	// 使用更兼容的查询，不依赖json_valid函数
	err := vdb.db.QueryRow("SELECT embedding FROM file_chunks WHERE embedding IS NOT NULL AND embedding != '' AND embedding LIKE '[%]' LIMIT 1").Scan(&embeddingStr)
	if err == nil && embeddingStr != "" {
		// 成功获取为JSON格式
		var vecFromJson []float32
		if err := json.Unmarshal([]byte(embeddingStr), &vecFromJson); err == nil {
			fmt.Printf("检测到JSON存储格式，维度: %d\n", len(vecFromJson))
			vdb.dimension = len(vecFromJson)
		}
	}
}

// printDiagnosticInfo 输出诊断信息
func (vdb *VectorDBVec) printDiagnosticInfo() {
	fmt.Println("\n=== 向量数据库诊断信息 ===")
	fmt.Printf("向量维度: %d\n", vdb.dimension)
	fmt.Printf("可用的向量距离函数: %v\n", vdb.availableDistanceFuncs)
}

// SearchSimilarChunks 在 file_chunks 表的 embedding 字段做向量检索
func (vdb *VectorDBVec) SearchSimilarChunks(ctx context.Context, query []float32, limit int, fileIDList []string) ([]string, []float64, error) {
	startTime := time.Now()
	fmt.Printf("开始向量搜索，维度: %d, 限制: %d\n", len(query), limit)

	if vdb.dimension > 0 && vdb.dimension != len(query) {
		fmt.Printf("警告：查询向量维度 %d 与检测到的维度 %d 不匹配\n", len(query), vdb.dimension)
	}
	if len(vdb.availableDistanceFuncs) == 0 {
		return nil, nil, fmt.Errorf("没有可用的向量搜索方法，sqlite-vec扩展可能未正确加载")
	}

	var ids []string
	var dists []float64
	var err error
	var method string
	var rows *sql.Rows
	var idStr string
	var dist float64
	var duration time.Duration

	// 用sqlite-vec的compact格式序列化查询向量
	qBlob, err := sqlite_vec.SerializeFloat32(query)
	if err != nil {
		return nil, nil, fmt.Errorf("查询向量BLOB序列化失败: %w", err)
	}

	// 只用BLOB路径
	var distFunc string
	var sqlQuery string
	placeholders := strings.Repeat("?,", len(fileIDList))
	placeholders = placeholders[:len(placeholders)-1]
	whereClause := "embedding IS NOT NULL AND length(embedding) > 0"
	if len(fileIDList) > 0 {
		whereClause += fmt.Sprintf(" AND file_id IN (%s)", placeholders)
	}

	if contains(vdb.availableDistanceFuncs, "vec_distance_cosine") {
		distFunc = "vec_distance_cosine"
		sqlQuery = fmt.Sprintf(`
			SELECT rowid AS rowid, id AS uuid, vec_distance_cosine(embedding, ?) as distance
			FROM aog_rag_chunk
			WHERE %s
			ORDER BY distance
			LIMIT ?
		`, whereClause)
	} else if contains(vdb.availableDistanceFuncs, "vec_distance_L2") {
		distFunc = "vec_distance_L2"
		sqlQuery = fmt.Sprintf(`
			SELECT rowid AS rowid, id AS uuid, vec_distance_L2(embedding, ?) as distance
			FROM aog_rag_chunk
			WHERE %s
			ORDER BY distance
			LIMIT ?
		`, whereClause)
	}

	type chunkResult struct {
		rowid int64
		uuid  string
		dist  float64
	}
	var results []chunkResult
	// 检查SQL语句
	fmt.Println("生成的SQL语句:", sqlQuery)
	if sqlQuery != "" {
		args := []interface{}{qBlob}
		for _, id := range fileIDList {
			args = append(args, id)
		}
		args = append(args, limit)
		rows, err = vdb.db.QueryContext(ctx, sqlQuery, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var rowid int64
				var uuid string
				var dist float64
				if err := rows.Scan(&rowid, &uuid, &dist); err == nil {
					results = append(results, chunkResult{rowid, uuid, dist})
				}
			}
			if len(results) > 0 {
				method = "距离函数 " + distFunc
				duration = time.Since(startTime)
				// 拆分结果
				ids := make([]string, 0, len(results))
				dists := make([]float64, 0, len(results))
				for _, r := range results {
					ids = append(ids, r.uuid)
					dists = append(dists, r.dist)
				}
				fmt.Printf("向量搜索成功，使用方法: %s, 结果数: %d, 耗时: %v\n",
					method, len(ids), duration)
				return ids, dists, nil
			}
		}
	}
	// 降级文本检索
	fmt.Println("尝试降级到简单文本搜索...")
	simpleQuery := `
	SELECT rowid AS id, 0.5 as distance
	FROM aog_rag_chunk
	WHERE embedding IS NOT NULL AND length(embedding) > 0`

	if len(fileIDList) > 0 {
		args := []interface{}{}
		for _, id := range fileIDList {
			args = append(args, id)
		}
		args = append(args, limit)
		simpleQuery += fmt.Sprintf(" AND file_id IN (%s)", placeholders)
		simpleQuery += " ORDER BY id DESC LIMIT ?"
		rows, err = vdb.db.QueryContext(ctx, simpleQuery, args...)
	} else {
		simpleQuery += " ORDER BY id DESC LIMIT ?"
		rows, err = vdb.db.QueryContext(ctx, simpleQuery, limit)
	}

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			if err := rows.Scan(&idStr, &dist); err == nil {
				ids = append(ids, idStr)
				dists = append(dists, dist)
			}
		}
		if len(ids) > 0 {
			method = "降级简单文本"
			duration = time.Since(startTime)
			fmt.Printf("向量搜索成功，使用方法: %s, 结果数: %d, 耗时: %v\n",
				method, len(ids), duration)
			return ids, dists, nil
		}
	}

	// 所有向量方法都失败
	err = fmt.Errorf("所有向量搜索方法均失败")
	duration = time.Since(startTime)
	fmt.Printf("向量搜索失败: %v, 耗时: %v\n", err, duration)
	return nil, nil, err
}

// SearchSimilarChunksByText 使用文本搜索作为向量搜索的备选方案
func (vdb *VectorDBVec) SearchSimilarChunksByText(ctx context.Context, queryText string, limit int, fileIDList []string) ([]string, []float64, error) {
	startTime := time.Now()
	fmt.Printf("开始文本搜索降级，查询文本: %s, 限制: %d\n", queryText, limit)

	// 构造基本SQL查询
	var whereClause string
	if len(fileIDList) > 0 {
		whereClause = `
		content IS NOT NULL 
		AND (
			content LIKE ? OR  -- 精确匹配
			content LIKE ? OR  -- 包含查询词
			content LIKE ?     -- 包含查询词的一部分
		)
		AND file_id IN (?)
		`
	} else {
		whereClause = `
		content IS NOT NULL 
		AND (
			content LIKE ? OR  -- 精确匹配
			content LIKE ? OR  -- 包含查询词
			content LIKE ?     -- 包含查询词的一部分
		)
		`
	}

	// 使用简单的文本匹配，按相关度降序排序
	sqlQuery := fmt.Sprintf(`
		SELECT id, 
		-- 计算简单的文本匹配分数，越小越相关
		(CASE 
			WHEN content LIKE ? THEN 0.1  -- 精确匹配
			WHEN content LIKE ? THEN 0.3  -- 包含查询词
			WHEN content LIKE ? THEN 0.5  -- 包含查询词的一部分
			ELSE 0.9                      -- 其他情况
		END) as distance
		FROM aog_rag_chunk
		WHERE %s
		ORDER BY distance
		LIMIT ?
	`, whereClause)
	exactMatch := queryText
	containsMatch := "%" + queryText + "%"
	partialMatch := "%" + strings.Join(strings.Fields(queryText), "%") + "%"

	var rows *sql.Rows
	var err error

	// 根据是否有会话ID使用不同参数查询
	rows, err = vdb.db.QueryContext(ctx, sqlQuery,
		exactMatch, containsMatch, partialMatch, // 用于CASE计算
		exactMatch, containsMatch, partialMatch, // 用于WHERE过滤
		limit, strings.Join(fileIDList, ","))
	if err != nil {
		return nil, nil, fmt.Errorf("文本搜索查询失败: %w", err)
	}
	defer rows.Close()

	var ids []string
	var dists []float64

	for rows.Next() {
		var idStr string
		var dist float64
		if err := rows.Scan(&idStr, &dist); err == nil {
			ids = append(ids, idStr)
			dists = append(dists, dist)
		}
	}

	duration := time.Since(startTime)
	if len(ids) == 0 {
		fmt.Printf("文本搜索未找到结果，耗时: %v\n", duration)
		return nil, nil, fmt.Errorf("文本搜索未找到结果")
	}

	fmt.Printf("文本搜索成功，结果数: %d, 耗时: %v\n", len(ids), duration)
	return ids, dists, nil
}

// DeleteChunks 从数据库中删除指定ID的文件块
func (vdb *VectorDBVec) DeleteChunks(ctx context.Context, chunkIDs []string) error {
	if vdb.db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}

	startTime := time.Now()
	fmt.Printf("开始删除文件块，数量: %d\n", len(chunkIDs))

	// 构建SQL批量删除语句
	placeholders := make([]string, len(chunkIDs))
	args := make([]interface{}, len(chunkIDs))
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	// 删除文件块记录
	sqlQuery := fmt.Sprintf("DELETE FROM file_chunks WHERE id IN (%s)", strings.Join(placeholders, ","))
	result, err := vdb.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("删除文件块失败: %w", err)
	}

	// 获取删除的行数
	rowsAffected, _ := result.RowsAffected()
	duration := time.Since(startTime)
	fmt.Printf("删除文件块完成，影响行数: %d, 耗时: %v\n", rowsAffected, duration)

	return nil
}

// GetChunkByID 通过主键 id 精确查询 file_chunks 内容
func (vdb *VectorDBVec) GetChunkByID(ctx context.Context, id string) (content string, chunkID string, err error) {
	var c string
	var cid string
	err = vdb.db.QueryRowContext(ctx, "SELECT id, content FROM aog_rag_chunk WHERE id = ? LIMIT 1", id).Scan(&cid, &c)
	if err != nil {
		return "", "", err
	}
	return c, cid, nil
}

func InitVecDB(dbPath string) error {
	VecMutex.Lock()
	defer VecMutex.Unlock()

	if VecInitialized {
		return nil
	}

	var err error
	VecDB, err = NewVectorDBVec(dbPath)
	if err != nil {
		return fmt.Errorf("初始化VEC数据库失败: %w", err)
	}

	// 初始化VEC表结构
	if err := VecDB.Initialize(); err != nil {
		return fmt.Errorf("初始化VEC表结构失败: %w", err)
	}

	VecInitialized = true
	logger.LogicLogger.Info("VEC向量数据库初始化成功")
	return nil
}

package bcode

import "net/http"

var (
	RagSuccessCode = NewBcode(http.StatusOK, 60000, "rag service interface call success")

	ErrRagBadRequest = NewBcode(http.StatusBadRequest, 60001, " bad request")

	ErrRagFileSize = NewBcode(http.StatusBadRequest, 60002, "file size too large")

	ErrRagFileType = NewBcode(http.StatusBadRequest, 60003, "Unsupported file type")

	ErrRagServerError = NewBcode(http.StatusInternalServerError, 60004, "Internal server error")

	ErrRagFileStatus = NewBcode(http.StatusBadRequest, 60005, "File status is invalid")

	ErrRagSqliteVec = NewBcode(http.StatusBadRequest, 60006, "vec db status error")

	ErrRagEmbedding = NewBcode(http.StatusBadRequest, 60007, "embedding error")

	ErrRagRetrieval = NewBcode(http.StatusBadRequest, 60008, "retrieval error")

	ErrRagAOGBaseService = NewBcode(http.StatusBadRequest, 60009, "AOG service error")
)

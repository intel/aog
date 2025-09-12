package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/utils/bcode"
)

func (t *AOGCoreServer) RagGetFile(c *gin.Context) {
	request := new(dto.RagGetFileRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	data, err := t.RagService.GetFile(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) RagGetFiles(c *gin.Context) {
	ctx := c.Request.Context()
	data, err := t.RagService.GetFiles(ctx)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) RagUploadFile(c *gin.Context) {
	data, err := t.RagService.UploadFile(c)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) RagDeleteFile(c *gin.Context) {
	request := new(dto.RagDeleteFileRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	data, err := t.RagService.DeleteFile(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) RagRetrieval(c *gin.Context) {
	request := new(dto.RagRetrievalRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	data, err := t.RagService.Retrieval(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

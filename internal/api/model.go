package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"intel.com/aog/internal/api/dto"
	"intel.com/aog/internal/utils/bcode"
)

func (t *AOGCoreServer) CreateModel(c *gin.Context) {
	request := new(dto.CreateModelRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		if !errors.Is(err, io.EOF) {
			bcode.ReturnError(c, bcode.ErrModelBadRequest)
			return
		}
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.Model.CreateModel(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) DeleteModel(c *gin.Context) {
	request := new(dto.DeleteModelRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.Model.DeleteModel(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) GetModels(c *gin.Context) {
	request := new(dto.GetModelsRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.Model.GetModels(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

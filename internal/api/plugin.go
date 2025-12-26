package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/utils/bcode"
)

func (t *AOGCoreServer) PluginInfo(c *gin.Context) {
	request := new(dto.GetPluginInfoRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	data, err := t.Plugin.PluginInfo(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) PluginList(c *gin.Context) {
	ctx := c.Request.Context()
	data, err := t.Plugin.PluginList(ctx)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) PluginDelete(c *gin.Context) {
	request := new(dto.PluginDeleteRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}
	ctx := c.Request.Context()
	data, err := t.Plugin.PluginDelete(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) PluginStop(c *gin.Context) {
	request := new(dto.PluginStopRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	data, err := t.Plugin.PluginStop(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) PluginLoad(c *gin.Context) {
	request := new(dto.PluginLoadRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	data, err := t.Plugin.PluginLoad(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) PluginDownload(c *gin.Context) {
	request := new(dto.PluginLoadRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	data, err := t.Plugin.PluginDownload(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

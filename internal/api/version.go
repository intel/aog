package api

import (
	"github.com/gin-gonic/gin"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/utils/bcode"
	"net/http"
)

func (t *AOGCoreServer) GetVersion(c *gin.Context) {
	logger.ApiLogger.Debug("[API] Get version request params:", c.Request.Body)

	ctx := c.Request.Context()
	resp, err := t.Health.HealthHeader(ctx)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] Get server health response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) GetEngineVersion(c *gin.Context) {
	logger.ApiLogger.Debug("[API] Get engine server health request params:", c.Request.Body)
	request := new(dto.GetEngineVersionRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	ctx := c.Request.Context()
	resp, err := t.Version.GetEngineVersion(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] Get engine server health response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) GetUpdateAvailableStatus(c *gin.Context) {
	ctx := c.Request.Context()
	resp, err := t.Version.UpdateAvailableHandler(ctx)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] Get update available status response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) UpdateHandler(c *gin.Context) {
	logger.ApiLogger.Debug("[API] Update AOG version params:", c.Request.Body)
	ctx := c.Request.Context()
	resp, err := t.Version.UpdateHandler(ctx)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] Update AOG version response:", resp)
	c.JSON(http.StatusOK, resp)

}

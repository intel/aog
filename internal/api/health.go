package api

import (
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/utils/bcode"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (t *AOGCoreServer) GetServerHealth(c *gin.Context) {
	logger.ApiLogger.Debug("[API] Get server health request params:", c.Request.Body)

	ctx := c.Request.Context()
	resp, err := t.Health.HealthHeader(ctx)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] Get server health response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) GetEngineServerHealth(c *gin.Context) {
	logger.ApiLogger.Debug("[API] Get engine server health request params:", c.Request.Body)
	request := new(dto.GetEngineHealthRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	ctx := c.Request.Context()
	resp, err := t.Health.EngineHealthHandler(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] Get engine server health response:", resp)
	c.JSON(http.StatusOK, resp)
}

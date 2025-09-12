package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
)

func HealthHeader(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]string{"status": "UP"})
}

func EngineHealthHandler(c *gin.Context) {
	data := make(map[string]string)
	for _, modelEngineName := range types.SupportModelEngine {
		engine := provider.GetModelEngine(modelEngineName)
		err := engine.HealthCheck()
		if err != nil {
			data[modelEngineName] = "DOWN"
			continue
		}
		data[modelEngineName] = "UP"
	}
	c.JSON(http.StatusOK, data)
}

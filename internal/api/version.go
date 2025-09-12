package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/process"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/version"
)

func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]string{"version": version.AOGVersion, "spec_version": version.SpecVersion})
}

func GetEngineVersion(c *gin.Context) {
	ctx := c.Request.Context()
	data := make(map[string]string)
	for _, modelEngineName := range types.SupportModelEngine {
		engine := provider.GetModelEngine(modelEngineName)
		var respData types.EngineVersionResponse
		resp, err := engine.GetVersion(ctx, &respData)
		if err != nil {
			data[modelEngineName] = "get version failed"
			continue
		}
		data[modelEngineName] = resp.Version
	}

	c.JSON(http.StatusOK, data)
}

func UpdateAvailableHandler(c *gin.Context) {
	ctx := c.Request.Context()
	status, updateResp := version.IsNewVersionAvailable(ctx)
	if status {
		c.JSON(http.StatusOK, map[string]string{"message": fmt.Sprintf("Ollama version %s is ready to install", updateResp.UpdateVersion)})
	} else {
		c.JSON(http.StatusOK, map[string]string{"message": ""})
	}
}

func UpdateHandler(c *gin.Context) {
	// check and stop server if running
	manager, err := process.GetAOGProcessManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		return
	}

	if manager.IsProcessRunning() {
		pidFilePath := filepath.Join(config.GlobalEnvironment.RootDir, "aog.pid")
		if err := manager.StopProcessWithFallback(pidFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
			return
		}
	}
	// rm old version file
	aogFileName := "aog.exe"
	if runtime.GOOS != "windows" {
		aogFileName = "aog"
	}
	aogFilePath := filepath.Join(config.GlobalEnvironment.RootDir, aogFileName)
	err = os.Remove(aogFilePath)
	if err != nil {
		slog.Error("[Update] Failed to remove aog file %s: %v\n", aogFilePath, err)
		c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	}
	// install new version
	downloadPath := filepath.Join(config.GlobalEnvironment.RootDir, "download", aogFileName)
	err = os.Rename(downloadPath, aogFilePath)
	if err != nil {
		slog.Error("[Update] Failed to rename aog file %s: %v\n", downloadPath, err)
		c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	}
	// start server
	if err := manager.StartProcessDaemon(); err != nil {
		slog.Error("[Update] Failed to start AOG server: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		return
	}
	ds := datastore.GetDefaultDatastore()
	ctx := c.Request.Context()
	vr := &types.VersionUpdateRecord{}
	sortOption := []datastore.SortOption{
		{Key: "created_at", Order: -1},
	}
	versionRecoreds, err := ds.List(ctx, vr, &datastore.ListOptions{SortBy: sortOption})
	if err != nil {
		slog.Error("[Update] Failed to list versions: %v\n", err)
		c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	}
	versionRecord := versionRecoreds[0].(*types.VersionUpdateRecord)
	if versionRecord.Status == types.VersionRecordStatusInstalled {
		versionRecord.Status = types.VersionRecordStatusUpdated
	}
	err = ds.Put(ctx, versionRecord)
	if err != nil {
		slog.Error("[Update] Failed to update versions: %v\n", err)
		c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	}
	c.JSON(http.StatusOK, map[string]string{"message": ""})
}

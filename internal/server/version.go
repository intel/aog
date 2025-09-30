package server

import (
	"context"
	"fmt"

	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
	"github.com/intel/aog/version"
)

type Version interface {
	GetVersion(ctx context.Context) (*dto.GetVersionResponse, error)
	GetEngineVersion(ctx context.Context, request *dto.GetEngineVersionRequest) (*dto.GetEngineVersionResponse, error)
	UpdateAvailableHandler(ctx context.Context) (*dto.UpdateAvailableResponse, error)
	UpdateHandler(ctx context.Context) (*dto.UpdateAvailableResponse, error)
}

type VersionImpl struct {
	Ds  datastore.Datastore
	JDs datastore.JsonDatastore
}

func NewVersion() Version {
	return &VersionImpl{
		Ds: datastore.GetDefaultDatastore(),
	}
}

func (v *VersionImpl) GetVersion(ctx context.Context) (*dto.GetVersionResponse, error) {
	res := &dto.GetVersionResponseData{
		Version:     version.AOGVersion,
		SpecVersion: version.SpecVersion,
	}
	return &dto.GetVersionResponse{
		Bcode: *bcode.VersionCode,
		Data:  *res,
	}, nil
}

func (v *VersionImpl) GetEngineVersion(ctx context.Context, request *dto.GetEngineVersionRequest) (*dto.GetEngineVersionResponse, error) {
	data := make(map[string]string)
	if request.EngineName != "" {
		engine := provider.GetModelEngine(request.EngineName)
		var respData types.EngineVersionResponse
		resp, err := engine.GetVersion(ctx, &respData)
		if err != nil {
			data[request.EngineName] = "get version failed"
		} else {
			data[request.EngineName] = resp.Version
		}
	} else {
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
	}

	return &dto.GetEngineVersionResponse{
		Bcode: *bcode.VersionCode,
		Data:  data,
	}, nil
}

func (v *VersionImpl) UpdateAvailableHandler(ctx context.Context) (*dto.UpdateAvailableResponse, error) {
	status, updateResp := version.IsNewVersionAvailable(ctx)
	respMessage := ""
	if status {
		respMessage = fmt.Sprintf("Ollama version %s is ready to install", updateResp.UpdateVersion)
	}
	res := &dto.UpdateAvailableResponseData{
		Message: respMessage,
		Status:  status,
	}
	return &dto.UpdateAvailableResponse{
		Bcode: *bcode.VersionCode,
		Data:  *res,
	}, nil
}

func (v *VersionImpl) UpdateHandler(ctx context.Context) (*dto.UpdateAvailableResponse, error) {
	// todo()
	// check and stop server if running
	//manager, err := process.GetAOGProcessManager()
	//if err != nil {
	//	c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
	//	return
	//}
	//
	//if manager.IsProcessRunning() {
	//	pidFilePath := filepath.Join(config.GlobalEnvironment.RootDir, "aog.pid")
	//	if err := manager.StopProcessWithFallback(pidFilePath); err != nil {
	//		c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
	//		return
	//	}
	//}
	//// rm old version file
	//aogFileName := "aog.exe"
	//if runtime.GOOS != "windows" {
	//	aogFileName = "aog"
	//}
	//aogFilePath := filepath.Join(config.GlobalEnvironment.RootDir, aogFileName)
	//err = os.Remove(aogFilePath)
	//if err != nil {
	//	slog.Error("[Update] Failed to remove aog file %s: %v\n", aogFilePath, err)
	//	c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	//}
	//// install new version
	//downloadPath := filepath.Join(config.GlobalEnvironment.RootDir, "download", aogFileName)
	//err = os.Rename(downloadPath, aogFilePath)
	//if err != nil {
	//	slog.Error("[Update] Failed to rename aog file %s: %v\n", downloadPath, err)
	//	c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	//}
	//// start server
	//if err := manager.StartProcessDaemon(); err != nil {
	//	slog.Error("[Update] Failed to start AOG server: %v", err)
	//	c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
	//	return
	//}
	//ds := datastore.GetDefaultDatastore()
	//ctx := c.Request.Context()
	//vr := &types.VersionUpdateRecord{}
	//sortOption := []datastore.SortOption{
	//	{Key: "created_at", Order: -1},
	//}
	//versionRecoreds, err := ds.List(ctx, vr, &datastore.ListOptions{SortBy: sortOption})
	//if err != nil {
	//	slog.Error("[Update] Failed to list versions: %v\n", err)
	//	c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	//}
	//versionRecord := versionRecoreds[0].(*types.VersionUpdateRecord)
	//if versionRecord.Status == types.VersionRecordStatusInstalled {
	//	versionRecord.Status = types.VersionRecordStatusUpdated
	//}
	//err = ds.Put(ctx, versionRecord)
	//if err != nil {
	//	slog.Error("[Update] Failed to update versions: %v\n", err)
	//	c.JSON(http.StatusOK, map[string]string{"message": err.Error()})
	//}
	//c.JSON(http.StatusOK, map[string]string{"message": ""})
	return &dto.UpdateAvailableResponse{}, nil
}

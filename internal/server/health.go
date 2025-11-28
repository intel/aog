package server

import (
	"context"

	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
)

type Health interface {
	HealthHeader(ctx context.Context) (*dto.GetSeverHealthResponse, error)
	EngineHealthHandler(ctx context.Context, request *dto.GetEngineHealthRequest) (*dto.GetEngineHealthResponse, error)
}

type HealthImpl struct {
	Ds datastore.Datastore
}

func NewHealth() Health {
	return &HealthImpl{
		Ds: datastore.GetDefaultDatastore(),
	}
}

func (h *HealthImpl) HealthHeader(ctx context.Context) (*dto.GetSeverHealthResponse, error) {
	resp := map[string]string{"status": "UP"}
	return &dto.GetSeverHealthResponse{
		Bcode: *bcode.HealthCode,
		Data:  resp,
	}, nil
}

func (h *HealthImpl) EngineHealthHandler(ctx context.Context, request *dto.GetEngineHealthRequest) (*dto.GetEngineHealthResponse, error) {
	data := make(map[string]string)
	if request.EngineName != "" {
		engine, err := provider.GetModelEngine(request.EngineName)
		if err != nil {
			data[request.EngineName] = "NOT_FOUND"
		} else if err := engine.HealthCheck(ctx); err != nil {
			data[request.EngineName] = "DOWN"
		} else {
			data[request.EngineName] = "UP"
		}

	} else {
		for _, modelEngineName := range types.SupportModelEngine {
			engine, err := provider.GetModelEngine(modelEngineName)
			if err != nil {
				data[modelEngineName] = "NOT_FOUND"
				continue
			}
			err = engine.HealthCheck(ctx)
			if err != nil {
				data[modelEngineName] = "DOWN"
				continue
			}
			data[modelEngineName] = "UP"
		}
	}
	return &dto.GetEngineHealthResponse{
		Bcode: *bcode.HealthCode,
		Data:  data,
	}, nil
}

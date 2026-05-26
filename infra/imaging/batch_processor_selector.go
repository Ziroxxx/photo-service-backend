package imaging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"photo-service-back/domain/photo"
)

type BatchSelectorProcessor struct {
	mode          ProcessorMode
	local         photo.BatchPhotoProcessor
	cuda          photo.BatchPhotoProcessor
	cudaClient    *CudaClient
	healthTimeout time.Duration
}

func NewBatchSelectorProcessor(
	mode string,
	local photo.BatchPhotoProcessor,
	cuda photo.BatchPhotoProcessor,
	cudaClient *CudaClient,
	healthTimeout time.Duration,
) *BatchSelectorProcessor {
	processorMode := ProcessorMode(strings.ToLower(strings.TrimSpace(mode)))

	if processorMode != ProcessorModeAuto &&
		processorMode != ProcessorModeCuda &&
		processorMode != ProcessorModeLocal {
		processorMode = ProcessorModeAuto
	}

	return &BatchSelectorProcessor{
		mode:          processorMode,
		local:         local,
		cuda:          cuda,
		cudaClient:    cudaClient,
		healthTimeout: healthTimeout,
	}
}

func (p *BatchSelectorProcessor) ProcessBatch(
	ctx context.Context,
	inputs []photo.ProcessInput,
) ([]*photo.ProcessedPhoto, error) {
	switch p.mode {
	case ProcessorModeLocal:
		return p.local.ProcessBatch(ctx, inputs)

	case ProcessorModeCuda:
		if p.cuda == nil {
			return nil, fmt.Errorf("cuda processor is nil")
		}
		return p.cuda.ProcessBatch(ctx, inputs)

	case ProcessorModeAuto:
		if p.cuda != nil && p.isCudaAvailable(ctx) {
			result, err := p.cuda.ProcessBatch(ctx, inputs)
			if err == nil {
				return result, nil
			}

			slog.Warn("cuda batch processor failed, fallback to local", "error", err)
		}

		return p.local.ProcessBatch(ctx, inputs)

	default:
		return p.local.ProcessBatch(ctx, inputs)
	}
}

func (p *BatchSelectorProcessor) isCudaAvailable(ctx context.Context) bool {
	if p.cudaClient == nil {
		return false
	}

	healthCtx, cancel := context.WithTimeout(ctx, p.healthTimeout)
	defer cancel()

	health, err := p.cudaClient.Health(healthCtx)
	if err != nil {
		slog.Warn("cuda service health check failed", "error", err)
		return false
	}

	return health.Status == "ok" && health.CudaAvailable
}

var _ photo.BatchPhotoProcessor = (*BatchSelectorProcessor)(nil)

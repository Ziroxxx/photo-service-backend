package imaging

import (
	"context"
	"log/slog"
	"time"

	"photo-service-back/domain/photo"
)

type ProcessorMode string

const (
	ProcessorModeAuto  ProcessorMode = "auto"
	ProcessorModeCuda  ProcessorMode = "cuda"
	ProcessorModeLocal ProcessorMode = "local"
)

type PhotoProcessor interface {
	Process(ctx context.Context, input photo.ProcessInput) (*photo.ProcessedPhoto, error)
}

type SelectorProcessor struct {
	mode          ProcessorMode
	local         PhotoProcessor
	cuda          PhotoProcessor
	cudaClient    *CudaClient
	healthTimeout time.Duration
}

func NewSelectorProcessor(
	mode string,
	local PhotoProcessor,
	cuda PhotoProcessor,
	cudaClient *CudaClient,
	healthTimeout time.Duration,
) *SelectorProcessor {
	processorMode := ProcessorMode(mode)

	if processorMode != ProcessorModeAuto &&
		processorMode != ProcessorModeCuda &&
		processorMode != ProcessorModeLocal {
		processorMode = ProcessorModeAuto
	}

	return &SelectorProcessor{
		mode:          processorMode,
		local:         local,
		cuda:          cuda,
		cudaClient:    cudaClient,
		healthTimeout: healthTimeout,
	}
}

func (p *SelectorProcessor) Process(
	ctx context.Context,
	input photo.ProcessInput,
) (*photo.ProcessedPhoto, error) {
	switch p.mode {
	case ProcessorModeLocal:
		return p.local.Process(ctx, input)

	case ProcessorModeCuda:
		return p.cuda.Process(ctx, input)

	case ProcessorModeAuto:
		if p.isCudaAvailable(ctx) {
			result, err := p.cuda.Process(ctx, input)
			if err == nil {
				return result, nil
			}

			slog.Warn("cuda processor failed, fallback to local", "error", err)
		}

		return p.local.Process(ctx, input)

	default:
		return p.local.Process(ctx, input)
	}
}

func (p *SelectorProcessor) isCudaAvailable(ctx context.Context) bool {
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

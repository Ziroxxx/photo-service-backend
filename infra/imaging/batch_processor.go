package imaging

import (
	"context"

	"photo-service-back/domain/photo"
)

type BatchPhotoProcessor interface {
	ProcessBatch(ctx context.Context, inputs []photo.ProcessInput) ([]*photo.ProcessedPhoto, error)
}

type LocalBatchProcessor struct {
	local PhotoProcessor
}

func NewLocalBatchProcessor(local PhotoProcessor) *LocalBatchProcessor {
	return &LocalBatchProcessor{
		local: local,
	}
}

func (p *LocalBatchProcessor) ProcessBatch(
	ctx context.Context,
	inputs []photo.ProcessInput,
) ([]*photo.ProcessedPhoto, error) {
	results := make([]*photo.ProcessedPhoto, 0, len(inputs))

	for _, input := range inputs {
		result, err := p.local.Process(ctx, input)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

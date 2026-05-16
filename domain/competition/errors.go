package competition

import "errors"

var (
	ErrCompetitionNotFound          = errors.New("competition not found")
	ErrStageNotFound                = errors.New("stage not found")
	ErrForbiddenCompetitionWrite    = errors.New("forbidden")
	ErrInvalidCompetitionDates      = errors.New("invalid competition date range")
	ErrInvalidCompetitionStatus     = errors.New("invalid competition status")
	ErrInvalidStageDate             = errors.New("invalid stage date")
	ErrCompetitionSlugAlreadyExists = errors.New("competition slug already exists")
)

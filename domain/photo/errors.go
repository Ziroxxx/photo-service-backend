package photo

import "errors"

var (
	ErrPhotoNotFound         = errors.New("photo not found")
	ErrPhotoDeleted          = errors.New("photo deleted")
	ErrForbiddenPhotoRead    = errors.New("forbidden")
	ErrForbiddenPhotoWrite   = errors.New("forbidden")
	ErrForbiddenPhotoUpload  = errors.New("forbidden")
	ErrInvalidStage          = errors.New("invalid stage")
	ErrInvalidImage          = errors.New("invalid image")
	ErrInvalidBib            = errors.New("invalid bib")
	ErrPhotoBibNotFound      = errors.New("photo bib not found")
	ErrPhotoBibAlreadyExists = errors.New("photo bib already exists")
	ErrPhotoVersionNotFound  = errors.New("photo version not found")
	ErrEmptyPhotoIDs         = errors.New("photo ids are required")
	ErrTooManyFiles          = errors.New("too many files")
	ErrNoFilesProvided       = errors.New("no files provided")
)

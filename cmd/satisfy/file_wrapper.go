package satisfy

import "io"

type FileWrapper struct {
	io.ReadSeeker
}

func NewFileWrapper(inner io.ReadSeeker) *FileWrapper {
	return &FileWrapper{ReadSeeker: inner}
}

func (this *FileWrapper) Close() error {
	_, err := this.Seek(0, io.SeekStart)
	// If close becomes seek, we can let HTTP call close until the cows come home. (But do they actually ever come home?)
	return err
}

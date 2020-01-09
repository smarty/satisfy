package contracts

type ArchiveWriter interface {
	Write([]byte) (int, error)
	Close() error
	WriteHeader(name string, size int64)
}
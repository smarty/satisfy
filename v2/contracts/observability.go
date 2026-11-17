package contracts

import "log/slog"

// ProgressPhase names the long-running step a Progress report describes.
type ProgressPhase string

const (
	ProgressArchiving   ProgressPhase = "archiving"
	ProgressDownloading ProgressPhase = "downloading"
	ProgressExtracting  ProgressPhase = "extracting"
)

// Progress reports how many bytes of a single item have been processed.
// Reports arrive periodically while work is under way and once more, with
// Done set, when the item is finished.
type Progress struct {
	Phase   ProgressPhase
	Name    string
	Written int64
	Total   int64
	Done    bool
}

// ProgressFunc receives progress reports. It may be called from multiple
// goroutines and should return quickly. A nil ProgressFunc disables reporting.
type ProgressFunc func(Progress)

// Report calls the function when it is not nil.
func (this ProgressFunc) Report(progress Progress) {
	if this != nil {
		this(progress)
	}
}

// LoggerOrDiscard returns logger, or a logger that discards everything when
// logger is nil, so that library code is silent unless a caller opts in.
func LoggerOrDiscard(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return logger
}

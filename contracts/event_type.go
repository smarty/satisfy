package contracts

type EventType int

const (
	eventUnknown  EventType = iota
	EventProgress           // file-level progress during archive build or extraction
	EventInfo               // general informational message
	EventWarning            // non-fatal warning
	EventFailure            // individual item failure (e.g. one package in a batch)
)

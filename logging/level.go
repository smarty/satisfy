package logging

const (
	Info     Level = iota // default
	NoPrefix              // don't print date/time/file
	Warning
	Error
)

var levelFormat = []string{
	"",
	"",
	" [WARN]",
	" [Error]",
}

// Level represents a log severity level for categorizing log messages.
type Level int

// String returns the prefix string associated with the log level.
//
// Returns:
//   - string: the level prefix for log formatting.
func (this Level) String() string {
	return levelFormat[this]
}

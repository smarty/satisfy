package logging

const (
	Info     Level = iota // default value
	NoPrefix              // no date-time and file prefix
	Warning
	Error
)

var levelFormat = []string{
	"",
	"",
	" [WARN]",
	" [Error]",
}

type Level int

// String returns the prefix string associated with the log level.
//
// Returns:
//   - string: the level prefix for log formatting.
func (this Level) String() string {
	return levelFormat[this]
}

package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/smarty/satisfy/v2/internal/archive"
	"github.com/smarty/satisfy/v2/internal/model"
)

// formatTags renders a tag list for the `tags list` command. With asJSON it
// emits a JSON array (an empty list renders as "[]"). Otherwise it emits one
// "name<TAB>version" line per tag, sorted by name; an empty list renders as the
// empty string so that scripts see no output and a zero exit code.
func formatTags(tags []model.Tag, asJSON bool) (string, error) {
	sorted := append([]model.Tag(nil), tags...)
	slices.SortFunc(sorted, func(a, b model.Tag) int { return strings.Compare(a.Name, b.Name) })

	if asJSON {
		if sorted == nil {
			sorted = []model.Tag{}
		}
		raw, err := json.Marshal(sorted, jsontext.WithIndent("  "))
		if err != nil {
			return "", err
		}
		return string(raw) + "\n", nil
	}

	builder := new(strings.Builder)
	for _, tag := range sorted {
		_, _ = fmt.Fprintf(builder, "%s\t%s\n", tag.Name, tag.Version)
	}
	return builder.String(), nil
}

// progressRenderer draws progress reports on a terminal line, redrawing the
// line in place while an item is under way.
type progressRenderer struct {
	lock   sync.Mutex
	output io.Writer
}

func newProgressRenderer(output io.Writer) *progressRenderer {
	return &progressRenderer{output: output}
}

func (this *progressRenderer) Report(progress model.Progress) {
	this.lock.Lock()
	defer this.lock.Unlock()
	_, _ = fmt.Fprint(this.output, renderProgress(progress))
}

func renderProgress(progress model.Progress) string {
	written := archive.HumanFileSize(progress.Written)
	if progress.Done {
		return fmt.Sprintf("\033[2K\rDone %s %s (%s).\n", progress.Phase, progress.Name, written)
	}
	total := archive.HumanFileSize(progress.Total)
	return fmt.Sprintf("\033[2K\r%s %s: %s of %s.", capitalize(string(progress.Phase)), progress.Name, written, total)
}

func capitalize(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

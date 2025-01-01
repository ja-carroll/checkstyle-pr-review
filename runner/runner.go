package runner

import (
	"bytes"
	"checkstyle-review/checkstylexml"
	"checkstyle-review/comment"
	"checkstyle-review/diff"
	"checkstyle-review/github"
	"context"
	"errors"
)

// DiffService is an interface which get diff.
type DiffService interface {
	Diff(context.Context) ([]byte, error)
	Strip() int
}

var linesPerFile = make(map[string]map[int]*diff.Line)

func Run(ctx context.Context, diffService *github.PullRequest, checkStyleResults map[string][]*checkstylexml.CheckStyleErrorFormat) error {

	b, err := diffService.Diff(ctx)
	if err != nil {
		return err
	}
	fileDiffs, err := diff.ParseMultiFile(bytes.NewReader(b))
	if err != nil {
		return err
	}
	var errs []error
	createDiffMappingDataStructures(fileDiffs)
	filteredErrors := filterCheckStyleErrors(checkStyleResults)
	for _, res := range filteredErrors {
		newC := &comment.Comment{
			Result:   res,
			ToolName: "checkStyle",
		}
		err := diffService.Post(ctx, newC)
		if err != nil {
			return err
		}
	}

	return errors.Join(errs...)
}

func createDiffMappingDataStructures(fileDiffs []*diff.FileDiff) {
	for _, file := range fileDiffs {
		path := file.PathNew
		lines, ok := linesPerFile[path]
		if !ok {
			lines = make(map[int]*diff.Line)
		}

		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.LnumNew > 0 {
					lines[line.LnumNew] = line
				}
			}
		}

		linesPerFile[path] = lines

	}
}

func filterCheckStyleErrors(checkStyleResults map[string][]*checkstylexml.CheckStyleErrorFormat) []*checkstylexml.CheckStyleErrorFormat {
	var filterErrors = make([]*checkstylexml.CheckStyleErrorFormat, 0)
	for fileName, checkStyleResult := range checkStyleResults {
		_, ok := linesPerFile[fileName]
		if ok {
			for _, checkStyleErr := range checkStyleResult {
				newLine := checkStyleErr.Line
				_, isFine := linesPerFile[fileName][newLine]
				if isFine {
					filterErrors = append(filterErrors, checkStyleErr)
				}
			}
		}
	}
	return filterErrors
}

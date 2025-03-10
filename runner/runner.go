package runner

import (
	"bytes"
	"checkstyle-review/checkstylexml"
	"checkstyle-review/comment"
	"checkstyle-review/diff"
	"checkstyle-review/github"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	fmt.Printf("lines per file: %v\n", linesPerFile)
	filteredErrors := filterCheckStyleErrors(checkStyleResults)
	fmt.Printf("Filtered errors: %d\n", len(filteredErrors))
	postComments := make([]*comment.Comment, 0)
	for _, res := range filteredErrors {
		newC := &comment.Comment{
			Result:   res,
			ToolName: "checkStyle",
		}
		postComments = append(postComments, newC)
	}

	fmt.Printf("Posting comments: %d\n", len(postComments))
	err = diffService.PostAsReviewComment(ctx, postComments)
	if err != nil {
		return err
	}

	return errors.Join(errs...)
}

func createDiffMappingDataStructures(fileDiffs []*diff.FileDiff) {
	for _, file := range fileDiffs {
		path := normalizeDiffPath(file.PathNew, 1)
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

func normalizeDiffPath(diffpath string, strip int) string {
	path := diffpath
	if strip > 0 && !filepath.IsAbs(path) {
		ps := splitPathList(path)
		if len(ps) > strip {
			path = filepath.Join(ps[strip:]...)
		}
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func splitPathList(path string) []string {
	return strings.Split(filepath.ToSlash(path), "/")
}

func filterCheckStyleErrors(checkStyleResults map[string][]*checkstylexml.CheckStyleErrorFormat) []*checkstylexml.CheckStyleErrorFormat {
	cwd, _ := os.Getwd()
	var filterErrors = make([]*checkstylexml.CheckStyleErrorFormat, 0)
	for fileName, checkStyleResult := range checkStyleResults {
		fmt.Printf("Before it was normalized: %s\n", fileName)
		pathFileName := github.NormalizePath(fileName, cwd, "")
		fmt.Printf("Filter file name: %s\n", pathFileName)
		_, ok := linesPerFile[pathFileName]
		if ok {
			for _, checkStyleErr := range checkStyleResult {
				newLine := checkStyleErr.Line
				_, isFine := linesPerFile[pathFileName][newLine]
				if isFine {
					filterErrors = append(filterErrors, checkStyleErr)
				}
			}
		}
	}
	return filterErrors
}

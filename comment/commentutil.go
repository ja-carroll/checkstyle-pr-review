package comment

import (
	"checkstyle-review/checkstylexml"
	"fmt"
	"github.com/google/uuid"
	"strings"
)

// Comment represents a reported result as a comment.
type Comment struct {
	Result   *checkstylexml.CheckStyleErrorFormat
	ToolName string
}

type PostedComments map[uuid.UUID]struct{}

// IsPosted returns true if a given comment has been posted in code review service already,
// otherwise returns false. It sees comments with same path, same position,
// and same body as same comments.
func (p PostedComments) IsPosted(key uuid.UUID) bool {
	if _, ok := p[key]; !ok {
		return false
	}
	return true
}

func (p PostedComments) AddPostedComment(key uuid.UUID) {
	if _, ok := p[key]; !ok {
		p[key] = struct{}{}
	}
}

// MarkdownComment creates comment body markdown.
func MarkdownComment(c *Comment) string {
	var sb strings.Builder
	if s := parseSeverity(c); s != "" {
		sb.WriteString(s)
		sb.WriteString(" ")
	}
	if code := c.Result.Source; code != "" {
		sb.WriteString(fmt.Sprintf("<%s> ", code))
	}
	sb.WriteString(c.Result.Message)
	return sb.String()
}

func parseSeverity(c *Comment) string {
	s := c.Result.Severity
	switch s {
	case "error", "ERROR", "Error", "e", "E":
		return "🚫"
	case "warning", "WARNING", "Warning", "w", "W":
		return "⚠️"
	case "info", "INFO", "Info", "i", "I",
		"note", "NOTE", "Note", "n", "N": // Treat note as info.
		return "📝"
	default:
		return ""
	}
}

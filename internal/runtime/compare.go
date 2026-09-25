package runtime

import (
	"fmt"
	"strings"
)

const maxCompareLines = 1200

type FileCompareRange struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	LineCount int    `json:"line_count"`
}

type FileCompareResult struct {
	Equal     bool             `json:"equal"`
	Left      FileCompareRange `json:"left"`
	Right     FileCompareRange `json:"right"`
	Diff      string           `json:"diff,omitempty"`
	Truncated bool             `json:"truncated,omitempty"`
}

type lineDiffOp struct {
	prefix byte
	text   string
}

func (r *Runtime) CompareFiles(leftPath string, leftStart, leftEnd int, rightPath string, rightStart, rightEnd int, maxBytes, maxOutputBytes int) (FileCompareResult, error) {
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	if maxOutputBytes <= 0 {
		maxOutputBytes = 256 * 1024
	}

	leftText, err := r.ReadLines(leftPath, leftStart, leftEnd, maxBytes)
	if err != nil {
		return FileCompareResult{}, fmt.Errorf("read left range: %w", err)
	}
	rightText, err := r.ReadLines(rightPath, rightStart, rightEnd, maxBytes)
	if err != nil {
		return FileCompareResult{}, fmt.Errorf("read right range: %w", err)
	}

	leftLines := compareLines(leftText)
	rightLines := compareLines(rightText)
	if len(leftLines) > maxCompareLines || len(rightLines) > maxCompareLines {
		return FileCompareResult{}, fmt.Errorf("selected range exceeds max_compare_lines=%d", maxCompareLines)
	}

	leftStart = normalizedStartLine(leftStart)
	rightStart = normalizedStartLine(rightStart)
	result := FileCompareResult{
		Equal: leftText == rightText,
		Left: FileCompareRange{
			Path:      cleanRelative(leftPath),
			StartLine: leftStart,
			EndLine:   actualEndLine(leftStart, len(leftLines)),
			LineCount: len(leftLines),
		},
		Right: FileCompareRange{
			Path:      cleanRelative(rightPath),
			StartLine: rightStart,
			EndLine:   actualEndLine(rightStart, len(rightLines)),
			LineCount: len(rightLines),
		},
	}
	if result.Equal {
		return result, nil
	}

	ops := compareLineDiff(leftLines, rightLines)
	result.Diff, result.Truncated = renderCompareDiff(result.Left, result.Right, ops, maxOutputBytes)
	return result, nil
}

func normalizedStartLine(line int) int {
	if line <= 0 {
		return 1
	}
	return line
}

func actualEndLine(start, lineCount int) int {
	if lineCount <= 0 {
		return 0
	}
	return start + lineCount - 1
}

func compareLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func compareLineDiff(left, right []string) []lineDiffOp {
	cols := len(right) + 1
	matrix := make([]uint16, (len(left)+1)*cols)
	for i := len(left) - 1; i >= 0; i-- {
		for j := len(right) - 1; j >= 0; j-- {
			at := i*cols + j
			if left[i] == right[j] {
				matrix[at] = matrix[(i+1)*cols+j+1] + 1
				continue
			}
			down := matrix[(i+1)*cols+j]
			across := matrix[i*cols+j+1]
			if down >= across {
				matrix[at] = down
			} else {
				matrix[at] = across
			}
		}
	}

	ops := make([]lineDiffOp, 0, len(left)+len(right))
	for i, j := 0, 0; i < len(left) || j < len(right); {
		if i < len(left) && j < len(right) && left[i] == right[j] {
			ops = append(ops, lineDiffOp{prefix: ' ', text: left[i]})
			i++
			j++
			continue
		}
		if i < len(left) && (j == len(right) || matrix[(i+1)*cols+j] >= matrix[i*cols+j+1]) {
			ops = append(ops, lineDiffOp{prefix: '-', text: left[i]})
			i++
			continue
		}
		ops = append(ops, lineDiffOp{prefix: '+', text: right[j]})
		j++
	}
	return ops
}

func renderCompareDiff(left, right FileCompareRange, ops []lineDiffOp, maxOutputBytes int) (string, bool) {
	var out strings.Builder
	lines := []string{
		fmt.Sprintf("--- %s:%d-%d", left.Path, left.StartLine, left.EndLine),
		fmt.Sprintf("+++ %s:%d-%d", right.Path, right.StartLine, right.EndLine),
		fmt.Sprintf("@@ -%d,%d +%d,%d @@", left.StartLine, left.LineCount, right.StartLine, right.LineCount),
	}
	for _, line := range lines {
		if !appendCompareLine(&out, line, maxOutputBytes) {
			return out.String(), true
		}
	}
	for _, op := range ops {
		if !appendCompareLine(&out, string(op.prefix)+op.text, maxOutputBytes) {
			return out.String(), true
		}
	}
	return out.String(), false
}

func appendCompareLine(out *strings.Builder, line string, maxOutputBytes int) bool {
	required := len(line) + 1
	if out.Len()+required > maxOutputBytes {
		return false
	}
	out.WriteString(line)
	out.WriteByte('\n')
	return true
}

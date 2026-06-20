// Parse sacct output into a struct

package slurm

import (
	"regexp"
	"strings"
)

// ANSI escape sequence regex (local to this file)
var ansiRegexSacct = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

type Sacct struct {
	Jobs []JobInfo
}

// cleanField strips ANSI codes, carriage returns, and control characters from
// a sacct output field.
func cleanField(field string) string {
	field = stripANSI(field)
	field = strings.ReplaceAll(field, "\r", "")
	field = strings.Map(func(r rune) rune {
		if r >= 32 || r == '\t' {
			return r
		}
		return -1
	}, field)
	return strings.TrimSpace(field)
}

// ParseParsable parses sacct output in --parsable2 format (pipe-delimited).
func (s *Sacct) ParseParsable(output string) error {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return nil
	}

	// First line is the header
	var fieldNames []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fieldNames = strings.Split(line, "|")
		break
	}

	if len(fieldNames) == 0 {
		return nil
	}

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "|")

		// Skip job-step lines (e.g. "12345.batch", "12345.extern", "12345.interactive")
		if len(fields) > 0 {
			jobID := fields[0]
			if strings.Contains(jobID, ".") {
				continue
			}
		}

		if len(fields) != len(fieldNames) {
			continue
		}

		s.Jobs = append(s.Jobs, parseJob(fields, fieldNames))
	}
	return nil
}

func parseJob(fields []string, fieldNames []string) JobInfo {
	m := make(map[string]string, len(fieldNames))
	for i, name := range fieldNames {
		if i < len(fields) {
			m[name] = fields[i]
		}
	}

	job := JobInfo{
		JobID:       cleanField(m["JobID"]),
		JobName:     cleanField(m["JobName"]),
		User:        cleanField(m["User"]),
		Account:     cleanField(m["Account"]),
		State:       stateFromString(m["State"]),
		StartTime:   cleanField(m["Start"]),
		ElapsedTime: cleanField(m["Elapsed"]),
		TimeLimit:   cleanField(m["Timelimit"]),
		AllocCPUS:   cleanField(m["AllocCPUS"]),
		AllocTRES:   cleanField(m["AllocTRES"]),
		NodeList:    cleanField(m["NodeList"]),
		StdOut:      cleanField(m["StdOut"]),
		StdErr:      cleanField(m["StdErr"]),
	}
	return job
}

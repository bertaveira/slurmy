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

func (s *Sacct) Parse(output string) error {
	var fieldNames []string

	for _, line := range strings.Split(output, "\n") {
		if line == "" || strings.Contains(line, "-----") {
			continue
		}
		if strings.Contains(line, "JobID") {
			fieldNames = strings.Fields(line)
			continue
		}

		fields := strings.Fields(line)

		// Skip job-step lines (e.g. "12345.batch", "12345.extern")
		if len(fields) > 0 && (strings.Contains(fields[0], ".ba") || strings.Contains(fields[0], ".ex")) {
			continue
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
	}
	return job
}


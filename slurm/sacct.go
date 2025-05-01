// Parse sacct output into a struct

package slurm

import (
	"fmt"
	"os/exec"
	"strings"
)

type Sacct struct {
	Jobs []JobInfo
}

func (s *Sacct) Parse(output string) error {
	lines := strings.Split(output, "\n")

	fieldNames := []string{}
	for _, line := range lines {
		if line == "" {
			continue
		}

		if strings.Contains(line, "JobID") {
			// Split line by spacing
			fieldNames = strings.Split(line, " ")
			// Print fields
			fmt.Println("Field names:", fieldNames)
			continue
		}

		fields := strings.Split(line, " ")
		job := parseJob(fields, fieldNames)
		s.Jobs = append(s.Jobs, job)
	}
	return nil
}

func parseJob(fields []string, fieldNames []string) JobInfo {
	job := JobInfo{}

	occupancy := [11]bool{}

	for i, field := range fields {
		fieldName := fieldNames[i]
		switch fieldName {
		case "JobID":
			job.JobID = field
			occupancy[0] = true
		case "JobName":
			job.JobName = field
			occupancy[1] = true
		case "User":
			job.User = field
			occupancy[2] = true
		case "Account":
			job.Account = field
			occupancy[3] = true
		case "State":
			job.State = stateFromString(field)
			occupancy[4] = true
		case "StartTime":
			job.StartTime = field
			occupancy[5] = true
		case "EndTime":
			job.EndTime = field
			occupancy[6] = true
		case "Elapsed":
			job.ElapsedTime = field
			occupancy[7] = true
		case "AllocCPUS":
			job.AllocCPUS = field
			occupancy[8] = true
		case "AllocTRES":
			job.AllocTRES = field
			occupancy[9] = true
		case "StdOut":
			job.StdOutFile = field
			occupancy[10] = true
		default:
			fmt.Println("Unknown field:", fieldName)
		}
	}

	for i, occupied := range occupancy {
		if !occupied {
			fmt.Println("Missing field:", fieldNames[i])
		}
	}
	return job
}

func RunSacct(user string) (*Sacct, error) {
	cmd := exec.Command("sacct", "--format=JobID,JobName,User,Account,State,StartTime,EndTime,Elapsed,AllocCPUS,AllocTRES,StdOut", "--name", user)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	sacct := &Sacct{}
	err = sacct.Parse(string(output))
	if err != nil {
		return nil, err
	}
	return sacct, nil
}

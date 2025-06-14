package git

import (
	"os/exec"
	"strings"
)

func gitCmdRun(path string, command string, args ...string) (string, error) {
	realArgs := append([]string{"-C", path, command}, args...)
	cmd := exec.Command("git", realArgs...)

	var buf strings.Builder
	cmd.Stdout = &buf
	err := cmd.Run()

	return buf.String(), err
}

func Branches(path string, needLocal bool, needRemote bool) ([]string, string, error) {
	output, err := gitCmdRun(path, "branch", "-a")
	if err != nil {
		return nil, "", err
	}

	var branchNames []string
	var currBranchName string
	for _, line := range strings.Split(output, "\n") {
		if len(line) < 2 {
			continue
		}

		branchName := strings.TrimSpace(line[2:])
		remote := false
		if strings.HasPrefix(branchName, "remotes/") {
			branchName = branchName[8:]
			remote = true
		}

		if needLocal && !remote || needRemote && remote {
			branchNames = append(branchNames, branchName)
		}

		if line[0] == '*' {
			currBranchName = branchName
		}
	}

	return branchNames, currBranchName, nil
}

func LogBetween(path string, fromBranch string, toBranch string) (string, error) {
	return gitCmdRun(path, "log", fromBranch+".."+toBranch, "-z")
}

func LogBetweenCount(path string, fromBranch string, toBranch string) (int, error) {
	log, err := LogBetween(path, fromBranch, toBranch)
	if err != nil {
		return 0, err
	}
	if log == "" {
		return 0, nil
	}
	return strings.Count(log, "\000") + 1, nil
}

func IsDirty(path string) (bool, error) {
	status, err := gitCmdRun(path, "status", "--short")
	if err != nil {
		return false, err
	}

	return status != "", nil
}

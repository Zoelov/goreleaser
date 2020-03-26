package commitvalid

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/apex/log"
	"github.com/fatih/color"
	"github.com/goreleaser/goreleaser/internal/git"
)

type Pipe struct{}

func (Pipe) String() string {
	return "commit message valider"
}

func (Pipe) Run() error {
	entries, err := getGitLog()
	if err != nil {
		return err
	}

	for _, commitMsg := range entries {
		splitMsgs := strings.SplitN(commitMsg, " ", 2)
		message := splitMsgs[1]
		err = valid(message)
		if err != nil {
			msg := fmt.Sprintf(" %v -- invalid", commitMsg)
			return errors.New(msg)
		}

		log.Debugf("%v passed.", commitMsg)
	}

	return nil
}

func getGitLog() ([]string, error) {
	var args = []string{"log", "--pretty=oneline", "--abbrev-commit", "--no-color", "--max-count=20"}
	logs, err := git.Run(args...)
	if err != nil {
		return nil, err
	}

	var entries = strings.Split(logs, "\n")
	return entries[0 : len(entries)-1], nil
}

type CommitType string

const (
	FEAT     CommitType = "feat"
	FIX      CommitType = "fix"
	DOCS     CommitType = "docs"
	STYLE    CommitType = "style"
	REFACTOR CommitType = "refactor"
	TEST     CommitType = "test"
	CHORE    CommitType = "chore"
	PERF     CommitType = "perf"
	HOTFIX   CommitType = "hotfix"
)

const CommitMessagePattern = `^(?:fixup!\s*)?(\w*)(\(([\w\$\.\*/-].*)\))?\: (.*)|^Merge\ branch(.*)`

func valid(message string) error {
	var commitMsgReg = regexp.MustCompile(CommitMessagePattern)

	commitTypes := commitMsgReg.FindAllStringSubmatch(message, -1)

	if len(commitTypes) != 1 {
		msg := color.New(color.Bold).Sprintf("[%v] not match", message)
		return errors.New(msg)
	} else {
		switch commitTypes[0][1] {
		case string(FEAT):
		case string(FIX):
		case string(DOCS):
		case string(STYLE):
		case string(REFACTOR):
		case string(TEST):
		case string(CHORE):
		case string(PERF):
		case string(HOTFIX):
		default:
			if !strings.HasPrefix(message, "Merge branch") {
				msg := color.New(color.Bold).Sprintf("[%v] not match", message)
				return errors.New(msg)
			}
		}
	}

	return nil
}

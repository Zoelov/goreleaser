// Package changelog provides the release changelog to goreleaser.
package changelog

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/goreleaser/goreleaser/internal/client"
	"github.com/goreleaser/goreleaser/internal/git"
	"github.com/goreleaser/goreleaser/internal/pipe"
	"github.com/goreleaser/goreleaser/pkg/context"
)

// ErrInvalidSortDirection happens when the sort order is invalid
var ErrInvalidSortDirection = errors.New("invalid sort direction")

// Pipe for checksums
type Pipe struct{}

func (Pipe) String() string {
	return "generating changelog"
}

// Run the pipe
func (Pipe) Run(ctx *context.Context) error {
	// TODO: should probably have a different field for the filename and its
	// contents.
	if ctx.ReleaseNotes != "" {
		notes, err := loadFromFile(ctx.ReleaseNotes)
		if err != nil {
			return err
		}
		log.WithField("file", ctx.ReleaseNotes).Info("loaded custom release notes")
		log.WithField("file", ctx.ReleaseNotes).Debugf("custom release notes: \n%s", notes)
		ctx.ReleaseNotes = notes
	}
	if ctx.Config.Changelog.Skip {
		return pipe.Skip("changelog should not be built")
	}
	if ctx.Snapshot {
		return pipe.Skip("not available for snapshots")
	}
	if ctx.ReleaseNotes != "" {
		return nil
	}
	if ctx.ReleaseHeader != "" {
		header, err := loadFromFile(ctx.ReleaseHeader)
		if err != nil {
			return err
		}
		ctx.ReleaseHeader = header
	}
	if ctx.ReleaseFooter != "" {
		footer, err := loadFromFile(ctx.ReleaseFooter)
		if err != nil {
			return err
		}
		ctx.ReleaseFooter = footer
	}

	if err := checkSortDirection(ctx.Config.Changelog.Sort); err != nil {
		return err
	}

	entries, err := buildChangelog(ctx)
	if err != nil {
		return err
	}

	changelogStringJoiner := "\n"
	if ctx.TokenType == context.TokenTypeGitLab || ctx.TokenType == context.TokenTypeGitea {
		// We need two or more whitespace to let markdown interpret
		// it as newline. See https://docs.gitlab.com/ee/user/markdown.html#newlines for details
		log.Debug("is gitlab or gitea changelog")
		changelogStringJoiner = "   \n"
	}

	tag := ctx.Git.CurrentTag
	ctx.ReleaseNotes = strings.Join(
		[]string{
			ctx.ReleaseHeader,
			fmt.Sprintf("## Version %v", tag[1:]),
			time.Now().Format("2006-01-02 15:04:05"),
			"</br>\n",
			strings.Join(entries, changelogStringJoiner),
			ctx.ReleaseFooter,
		},
		"\n\n",
	)

	var path = filepath.Join(ctx.Config.Dist, "CHANGELOG.md")
	log.WithField("changelog", path).Info("writing")
	return ioutil.WriteFile(path, []byte(ctx.ReleaseNotes), 0644)
}

func loadFromFile(file string) (string, error) {
	bts, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bts), nil
}

func checkSortDirection(mode string) error {
	switch mode {
	case "":
		fallthrough
	case "asc":
		fallthrough
	case "desc":
		return nil
	}
	return ErrInvalidSortDirection
}

func buildChangelog(ctx *context.Context) ([]string, error) {
	log, err := getChangelog(ctx.Git.CurrentTag)
	if err != nil {
		return nil, err
	}
	var entries = strings.Split(log, "\n")
	entries = entries[0 : len(entries)-1]
	entries, err = filterEntries(ctx, entries)
	if err != nil {
		return entries, err
	}
	entries, err = formatChangelog(ctx, entries)
	if err != nil {
		return entries, err
	}
	return sortEntries(ctx, entries), nil
}

func formatChangelog(ctx *context.Context, entries []string) ([]string, error) {
	c, err := client.New(ctx)
	if err != nil {
		return nil, err
	}

	fixList := make([]string, 0)
	fixList = append(fixList, "### 🐛Bug fixes")
	fixList = append(fixList, "***")

	featureList := make([]string, 0)
	featureList = append(featureList, "### 🚀Features")
	featureList = append(featureList, "***")

	choreList := make([]string, 0)
	choreList = append(choreList, "### 🔧Chores and Improvements")
	choreList = append(choreList, "***")

	otherList := make([]string, 0)
	otherList = append(otherList, "### 📦Other")
	otherList = append(otherList, "***")

	// font := "<font color=#0366d6 size=4 face=黑体>%v</font> "

	for _, v := range entries {
		splitStr := strings.SplitN(v, " ", 2)
		commitID := splitStr[0]
		log := splitStr[1]

		info, err := c.GetInfoByID(ctx, commitID)
		if err != nil {
			return nil, err
		}

		img := `<img src="%v" width="20" height="20" onclick=false title="%v"/>`
		img = fmt.Sprintf(img, info.AvatarURL, info.CommitterEmail)
		span := `<span style="display: inline-block;"> %v</span>`
		span = fmt.Sprintf(span, img)
		when := info.CommittedDate
		at := when.Format("2006-01-02 15:04:05")
		// avatar := fmt.Sprintf("![logo](%v)", info.AvatarURL)
		url := fmt.Sprintf("%v/%v/%v/commit/%v", info.BaseURL, ctx.Config.Release.GitLab.Owner, ctx.Config.Release.GitLab.Name, info.ID)
		prefix := fmt.Sprintf("__[%v](%v)__", commitID, url)

		if strings.HasPrefix(log, "fix:") {
			theLog := strings.Replace(log, "fix:", "", 1)
			theLog = strings.TrimSuffix(theLog[0:len(theLog)-1], " ")
			theLog = strings.TrimPrefix(theLog, " ")
			theLog = fmt.Sprintf(" ___%v___ ", theLog)
			email := fmt.Sprintf(" created by %v\n", span)
			suffix := fmt.Sprintf("*at:%v*\n", at)
			theLog = "* " + prefix + " " + theLog + email + suffix

			fixList = append(fixList, theLog)
		} else if strings.HasPrefix(log, "feat:") {
			theLog := strings.Replace(log, "feat:", "", 1)
			theLog = strings.TrimSuffix(theLog[0:len(theLog)-1], " ")
			theLog = strings.TrimPrefix(theLog, " ")
			theLog = fmt.Sprintf("___%v___", theLog)
			email := fmt.Sprintf(" created by %v\n", span)
			suffix := fmt.Sprintf("*at:%v*\n", at)
			theLog = "* " + prefix + " " + theLog + email + suffix

			featureList = append(featureList, theLog)
		} else if strings.HasPrefix(log, "chore:") {
			theLog := strings.Replace(log, "chore:", "", 1)
			theLog = strings.TrimSuffix(theLog[0:len(theLog)-1], " ")
			theLog = strings.TrimPrefix(theLog, " ")
			theLog = fmt.Sprintf("___%v___", theLog)
			email := fmt.Sprintf(" created by %v\n", span)
			suffix := fmt.Sprintf("*at:%v*\n", at)
			theLog = "* " + prefix + " " + theLog + email + suffix

			choreList = append(choreList, theLog)
		} else if strings.HasPrefix(log, "pref:") {
			theLog := strings.Replace(log, "pref:", "", 1)
			theLog = strings.TrimSuffix(theLog[0:len(theLog)-1], " ")
			theLog = strings.TrimPrefix(theLog, " ")
			theLog = fmt.Sprintf("___%v___", theLog)
			email := fmt.Sprintf(" created by %v\n", span)
			suffix := fmt.Sprintf("*at:%v*\n", at)
			theLog = "* " + prefix + " " + theLog + email + suffix

			choreList = append(choreList, theLog)
		} else {
			// theLog := log[0:len(log)-1] + "* " + fmt.Sprintf("@%v\n", info.AuthorEmail)
			theLog := strings.TrimSuffix(log[0:len(log)-1], " ")
			theLog = strings.TrimPrefix(theLog, " ")
			theLog = fmt.Sprintf("___%v___", theLog)
			email := fmt.Sprintf(" created by %v\n", span)
			suffix := fmt.Sprintf("*at:%v*\n", at)
			theLog = "* " + prefix + " " + theLog + email + suffix

			otherList = append(otherList, theLog)
		}
	}

	ret := make([]string, 0)
	if len(fixList) > 2 {
		ret = append(ret, fixList...)
		ret = append(ret, "<br/>\n")
	}
	if len(featureList) > 2 {
		ret = append(ret, featureList...)
		ret = append(ret, "<br/>\n")
	}

	if len(choreList) > 2 {
		ret = append(ret, choreList...)
		ret = append(ret, "<br/>\n")
	}

	if len(otherList) > 2 {
		ret = append(ret, otherList...)
		ret = append(ret, "<br/>\n")
	}
	return ret, nil
}

func filterEntries(ctx *context.Context, entries []string) ([]string, error) {
	for _, filter := range ctx.Config.Changelog.Filters.Exclude {
		r, err := regexp.Compile(filter)
		if err != nil {
			return entries, err
		}
		entries = remove(r, entries)
	}
	return entries, nil
}

func sortEntries(ctx *context.Context, entries []string) []string {
	var direction = ctx.Config.Changelog.Sort
	if direction == "" {
		return entries
	}
	var result = make([]string, len(entries))
	copy(result, entries)
	sort.Slice(result, func(i, j int) bool {
		var imsg = extractCommitInfo(result[i])
		var jmsg = extractCommitInfo(result[j])
		if direction == "asc" {
			return strings.Compare(imsg, jmsg) < 0
		}
		return strings.Compare(imsg, jmsg) > 0
	})
	return result
}

func remove(filter *regexp.Regexp, entries []string) (result []string) {
	for _, entry := range entries {
		if !filter.MatchString(extractCommitInfo(entry)) {
			result = append(result, entry)
		}
	}
	return result
}

func extractCommitInfo(line string) string {
	return strings.Join(strings.Split(line, " ")[1:], " ")
}

func getChangelog(tag string) (string, error) {
	prev, err := previous(tag)
	if err != nil {
		return "", err
	}
	if isSHA1(prev) {
		return gitLog(prev, tag)
	}
	return gitLog(fmt.Sprintf("tags/%s..tags/%s", prev, tag))
}

func gitLog(refs ...string) (string, error) {
	var args = []string{"log", "--pretty=oneline", "--abbrev-commit", "--no-decorate", "--no-color"}
	args = append(args, refs...)
	return git.Run(args...)
}

func previous(tag string) (result string, err error) {
	if tag := os.Getenv("GORELEASER_PREVIOUS_TAG"); tag != "" {
		return tag, nil
	}

	result, err = git.Clean(git.Run("describe", "--tags", "--abbrev=0", fmt.Sprintf("tags/%s^", tag)))
	if err != nil {
		result, err = git.Clean(git.Run("rev-list", "--max-parents=0", "HEAD"))
	}
	return
}

// nolint: gochecknoglobals
var validSHA1 = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

// isSHA1 te lets us know if the ref is a SHA1 or not
func isSHA1(ref string) bool {
	return validSHA1.MatchString(ref)
}

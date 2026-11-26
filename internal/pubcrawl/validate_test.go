package pubcrawl

import (
	"io"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func FuzzValidate(f *testing.F) {
	f.Add("project", "subscription", "topic", "lines", -1, int64(time.Second), false)
	f.Add("project", "projects/other/subscriptions/sub", "projects/other/topics/topic", "jsonl", 1, int64(1), false)
	f.Add("", "", "", "", 0, int64(0), false)
	f.Add("project", "projects/other/topics/sub", "projects/other/subscriptions/topic", "xml", -2, int64(-1), false)
	f.Add("project", "subscription", "topic", "lines", 1, int64(1), true)
	f.Add("project", "subscription", "topic", "lines", 0, int64(time.Second), false)
	f.Add("project", "subscription", "topic", "lines", -2, int64(time.Second), false)

	resourceID := regexp.MustCompile(`^[^/ \t\r\n]+$`)
	subscriptionName := regexp.MustCompile(`^projects/[^/ \t\r\n]+/subscriptions/[^/ \t\r\n]+$`)
	topicName := regexp.MustCompile(`^projects/[^/ \t\r\n]+/topics/[^/ \t\r\n]+$`)

	f.Fuzz(func(t *testing.T, project, subscription, topic, format string, limit int, idleNanos int64, conflict bool) {
		projectOK := resourceID.MatchString(project)
		subscriptionOK := resourceID.MatchString(subscription) || subscriptionName.MatchString(subscription)
		topicOK := resourceID.MatchString(topic) || topicName.MatchString(topic)
		formatOK := format == "lines" || format == "jsonl"
		receiveOK := (limit == -1 || limit > 0) && idleNanos > 0

		limitFlag := "--limit=" + strconv.Itoa(limit)
		idleFlag := "--idle-timeout=" + time.Duration(idleNanos).String()
		formatFlag := "--format=" + format
		publishFlags := []string{formatFlag}
		// Reuse the numeric input to exercise omitted, zero, positive, and negative rates.
		if limit != -1 {
			publishFlags = append(publishFlags, "--rate="+strconv.Itoa(limit))
		}
		if conflict {
			publishFlags = append(publishFlags, "--message=payload")
		}

		for _, tc := range []struct {
			name  string
			flags []string
			args  []string
			valid bool
		}{
			{"pull", []string{limitFlag, idleFlag, formatFlag}, []string{subscription}, projectOK && subscriptionOK && receiveOK && formatOK},
			{"publish", publishFlags, []string{topic}, projectOK && topicOK && formatOK && !conflict && limit >= -1},
			{"move", []string{limitFlag, idleFlag}, []string{subscription, topic}, projectOK && subscriptionOK && topicOK && receiveOK},
		} {
			root := NewCommand("")
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			args := []string{tc.name, "--project=" + project}
			args = append(args, tc.flags...)
			args = append(args, "--")
			args = append(args, tc.args...)

			command, _, err := root.Find(args)
			if err != nil || command.Name() != tc.name {
				t.Fatalf("find %s: command = %v, error = %v", tc.name, command, err)
			}

			// Execute Cobra's validation with a stub in place of the I/O.
			ran := false
			command.RunE = func(_ *cobra.Command, _ []string) error {
				ran = true
				return nil
			}
			root.SetArgs(args)

			if err := root.Execute(); (err == nil) != tc.valid || ran != tc.valid {
				t.Fatalf("%s: error = %v, handler ran = %t, want valid = %t", tc.name, err, ran, tc.valid)
			}
		}
	})
}

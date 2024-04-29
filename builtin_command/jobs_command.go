package builtin_command

import (
	"fmt"
	"oreshell/ast"
	"oreshell/job"
	"os"
)

var CommandNameJobs = "jobs"

// jobsコマンド
func Jobs(simpleCommand *ast.SimpleCommand) (err error) {
	args := simpleCommand.Args()
	l := len(args)
	if l == 0 {
		for _, v := range job.Jobs.ListJobString() {
			fmt.Fprintf(os.Stdout, "%s\n", v)
		}
	} else if l > 0 {
		return fmt.Errorf("%s: too many arguments", CommandNameJobs)
	}
	return nil
}

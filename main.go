package main

import (
	"bufio"
	"fmt"
	"io"
	"oreshell/ast"
	builtin "oreshell/builtin_command"
	"oreshell/job"
	"oreshell/lexer"
	"oreshell/log"
	"oreshell/myvariables"
	"oreshell/parser"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func init() {
	var err error
	log.Logger, err = log.NewForFile("oreshell.log")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var pgrpid int = 0

func getPgrpId() int {
	if pgrpid == 0 {
		pgrpid = job.Tcgetpgrp()
	}
	return pgrpid
}

// 外部コマンドを実行する
func execExternalCommand(pipelineSequence *ast.PipelineSequence) (err error) {

	j, err := job.NewJob(pipelineSequence)
	if err != nil {
		return err
	}
	err = j.Exec(getPgrpId())
	if err != nil {
		return err
	}

	return nil
}

func main() {

	signal.Ignore(syscall.SIGTSTP, syscall.SIGTTIN, syscall.SIGTTOU)

	// 標準入力から文字列を読み取る準備
	reader := bufio.NewReader(os.Stdin)

	// 内部コマンド群
	builtinCommands := map[string]func(*ast.SimpleCommand) error{
		builtin.CommandNameCd:     builtin.ChDir,
		builtin.CommandNameExport: builtin.ExportEnvironmentVariable,
		builtin.CommandNameExit:   builtin.Exit,
		builtin.CommandNameSet:    builtin.Set,
		builtin.CommandNameUnset:  builtin.Unset,
	}

	// ずっとループ
	for {
	L:
		for {
			select {
			// キューにメッセージが貯まっていれば、それを一つ取り出して
			case v := <-job.JobStatusNotify:
				log.Logger.Printf("jobstatusnotify %s", v)
				fmt.Fprintln(os.Stderr, v)
			// キューにメッセージがなければ
			default:
				break L
			}
		}

		// プロンプトを表示してユーザに入力を促す
		fmt.Printf("(ore) > ")

		// 標準入力から文字列(コマンド)を読み込む
		line, _, err := reader.ReadLine()
		if err != nil {
			// Ctrl+Dの場合
			if err == io.EOF {
				// 終了
				builtin.Exit(nil)
			} else {
				log.Logger.Fatalf("reader.ReadLine %v", err)
			}
		}

		// 入力文字列をトリム
		trimedL := strings.Trim(string(line), " ")
		if len(trimedL) == 0 {
			continue
		}

		// 字句解析
		l := lexer.Lex(trimedL)
		// 構文解析
		pipelineSequence, err := parser.NewParser().ParsePipelineSequence(l)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		log.Logger.Printf("pipelineSequence: %+v\n", pipelineSequence)

		// 外部/内部コマンドは実行せずに、シェル変数代入のみか
		if pipelineSequence.SimpleCommands[0].IsAssignVariablesOnly() {
			myvariables.Variables().AssignValuesToShellVariables(pipelineSequence.SimpleCommands[0].Variables())
		} else {
			// 先頭の単語に該当するコマンドを探して実行する
			// 内部コマンドか？
			builtinCommand, ok := builtinCommands[pipelineSequence.SimpleCommands[0].CommandName()]
			if ok {
				// 内部コマンドを実行
				err = builtinCommand(pipelineSequence.SimpleCommands[0])
			} else {
				// 外部コマンドを実行
				err = execExternalCommand(pipelineSequence)
			}
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

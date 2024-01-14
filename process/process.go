package process

import (
	"fmt"
	"oreshell/ast"
	"oreshell/log"
	"oreshell/myvariables"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	FD_DEFAULT_IN  = 0
	FD_DEFAULT_OUT = 1
	FD_DEFAULT_ERR = 2
)

const (
	FD_MIN = 0
	FD_MAX = 9
)

type Pipe struct {
	reader *os.File
	writer *os.File
}

type Process struct {
	command      string
	argv         []string
	stdin        *os.File
	stdout       *os.File
	redirections *[]ast.Redirection // FDをキー、入出力先ファイルを値とした辞書
	previous     *Process
	next         *Process
	pipe         *Pipe
	osProcess    *os.Process
	variablesMap map[string]string
	wg           *sync.WaitGroup
}

// 該当パスが存在するかどうか
func fileIsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// 指定された文字列が相対パスである場合、絶対パスを取得する。取得したパスが存在しなければエラーを返す。
// 指定された文字列がファイル名であるなら、環境変数PATHと連結して絶対パスを取得し存在すればそれを返す。存在しなければエラーを返す。
func absPathWithPATH(target string) (targetAbsPath string, err error) {

	// パスとファイル名を分離
	targetFileName := filepath.Base(target)
	//log.Logger.Printf("target %s\n", target)
	//log.Logger.Printf("targetFileName %s\n", targetFileName)

	// 指定された文字列がパスである場合
	if target != targetFileName {

		// 絶対パスの場合
		if filepath.IsAbs(target) {
			targetAbsPath = target
			// 相対パスの場合
		} else {
			targetAbsPath, err = filepath.Abs(target)
			if err != nil {
				log.Logger.Fatalf("filepath.Abs %v", err)
			}
		}

		if fileIsExist(targetAbsPath) {
			return targetAbsPath, nil
		} else {
			return "", fmt.Errorf("%s: no such file or directory", targetAbsPath)
		}
	}

	// 指定された文字列がファイル名である場合

	// 指定されたファイル名を環境変数パスの中から探す
	for _, path := range filepath.SplitList(os.Getenv("PATH")) {
		//log.Printf("%s\n", path)
		targetAbsPath = filepath.Join(path, targetFileName)
		if fileIsExist(targetAbsPath) {
			//log.Logger.Printf("find in PATH %s\n", targetAbsPath)
			return targetAbsPath, nil
		}
	}
	return "", fmt.Errorf("%s: no such file or directory", targetFileName)
}

func newPipe() (pipe *Pipe, err error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &Pipe{reader: reader, writer: writer}, nil
}

func (me *Pipe) close() {
	me.reader.Close()
	me.writer.Close()
}

func NewProcess(simpleCommand *ast.SimpleCommand, wg *sync.WaitGroup) (*Process, error) {
	// 該当するプログラムを探す
	command, err := absPathWithPATH(string(simpleCommand.CommandName()))
	if err != nil {
		return nil, err
	}
	log.Logger.Printf("command %s\n", command)
	log.Logger.Printf("argv : %v", simpleCommand.Argv())

	return &Process{
			command:      command,
			argv:         simpleCommand.Argv(),
			stdin:        os.Stdin,  // 初期値
			stdout:       os.Stdout, // 初期値
			redirections: simpleCommand.Redirections(),
			previous:     nil,
			next:         nil,
			pipe:         nil,
			osProcess:    nil,
			variablesMap: simpleCommand.Variables(),
			wg:           wg,
		},
		nil
}

func (me *Process) hasPrevious() bool {
	return me.previous != nil
}

func (me *Process) HasNext() bool {
	return me.next != nil
}

func (me *Process) hasPipe() bool {
	return me.pipe != nil
}

func (me *Process) isLeader() bool {
	return !me.hasPrevious()
}

func (me *Process) leader() *Process {
	p := me
	for p.hasPrevious() {
		p = p.previous
	}
	return p
}

func (me *Process) ToCommandString() (s string) {
	s = strings.Join(me.argv, " ")
	for _, v := range *me.redirections {
		if v.Direction() == ast.IN {
			s = fmt.Sprintf("%s < %s", s, v.FilePath())
		} else {
			s = fmt.Sprintf("%s > %s", s, v.FilePath())
		}
	}
	return s
}

func (me *Process) PipeWithNext(next *Process) (err error) {

	me.pipe, err = newPipe()
	if err != nil {
		return err
	}
	me.stdout = me.pipe.writer
	next.stdin = me.pipe.reader
	me.next = next
	next.previous = me

	return nil
}

func (me *Process) createProcAttrFiles() (files []*os.File, err error) {

	fdMap := map[int]*os.File{FD_DEFAULT_IN: me.stdin, FD_DEFAULT_OUT: me.stdout, FD_DEFAULT_ERR: os.Stderr}

	// redirectionsから辞書へ
	for _, v := range *me.redirections {
		var f *os.File
		if v.Direction() == ast.IN {
			// 入力用ファイルオープン
			f, err = os.Open(v.FilePath())
		} else { // ast.OUT
			// 出力用ファイルオープン
			f, err = os.Create(v.FilePath())
		}

		if err != nil {
			return nil, err
		}

		fdMap[v.FdNum()] = f
	}

	// 辞書からFileの配列へ
	files = []*os.File{}
	for i := FD_MIN; i <= FD_MAX; i++ {
		v, ok := fdMap[i]
		if ok {
			files = append(files, v)
		}
	}

	return files, nil
}

func (me *Process) createProcAttrEnv() (env []string) {

	assignVariableParser := myvariables.NewAssignVariableParser()

	// シェルプロセスの環境変数とユーザが設定した環境変数をマージ
	for _, v := range os.Environ() {
		log.Logger.Printf("createProcAttrEnv %v", v)
		_, name, value := assignVariableParser.TryParse(v)
		_, ok := me.variablesMap[name]
		if !ok { // ユーザ設定値を上書きしない
			me.variablesMap[name] = value
		}
	}

	// 環境変数のハッシュマップを配列に変換
	ar := []string{}
	for k, v := range me.variablesMap {
		ar = append(ar, fmt.Sprintf("%s=%s", k, v))
	}
	return ar

}

func (me *Process) Start(foreground bool) (err error) {
	var procAttr os.ProcAttr
	procAttr.Files, err = me.createProcAttrFiles()

	if me.isLeader() {
		procAttr.Sys = &syscall.SysProcAttr{Setpgid: true, Foreground: foreground}
	} else {
		log.Logger.Printf("leader pid: %v", me.leader().osProcess.Pid)
		procAttr.Sys = &syscall.SysProcAttr{Setpgid: true, Pgid: me.leader().osProcess.Pid, Foreground: foreground}
	}

	if me.variablesMap != nil || len(me.variablesMap) > 0 {
		procAttr.Env = me.createProcAttrEnv()
	}
	if err != nil {
		return err
	}

	log.Logger.Printf("me.argv : %v", me.argv)
	log.Logger.Printf("procAttr: %v", procAttr)
	log.Logger.Printf("procAttr.Files: %v", procAttr.Files)
	log.Logger.Printf("procAttr.Files[0].Name(): %v", procAttr.Files[0].Name())
	me.osProcess, err = os.StartProcess(me.command, me.argv, &procAttr)
	if err != nil {
		log.Logger.Fatalf("os.StartProcess %v", err)
		return err
	}
	log.Logger.Printf("me.commnd %v pid: %v", me.command, me.osProcess.Pid)

	go func() {
		ps, err := me.osProcess.Wait()
		if err != nil {
			log.Logger.Fatalf("process.Wait %v", err)
		} else {
			log.Logger.Printf("%v : ps.String() : %s", me.argv, ps.String())
			//log.Logger.Printf("ps.Sys() Signal(): %s", ps.Sys().(syscall.WaitStatus).Signal().String())
			//log.Logger.Printf("ps.Sys() Stopignal(): %s", ps.Sys().(syscall.WaitStatus).StopSignal().String())
			//log.Logger.Panicf("ps.String() : %#v", ps.SysUsage())
		}

		if me.hasPipe() {
			me.pipe.close()
		}

		me.wg.Done()
	}()

	return nil
}

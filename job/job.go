package job

import (
	"fmt"
	"oreshell/log"
	"oreshell/process"
	"strings"
	"sync"
)

type Job struct {
	processes   []*process.Process
	foreground  bool
	processesWg sync.WaitGroup
}

func (me *Job) processNum() int {
	if me.processes == nil {
		return 0
	} else {
		return len(me.processes)
	}
}

func (me *Job) toJobString() (s string) {
	var a []string // todo samber/loを使ってmap,reduceで書きたい
	for _, v := range me.processes {
		a = append(a, v.ToCommandString())
	}

	s = strings.Join(a, " | ")

	if !me.foreground {
		s = fmt.Sprintf("%s &", s)
	}

	return s
}

func (me *Job) Exec(pgrpid int) (err error) {

	err = me.start()
	if err != nil {
		return err
	}

	if me.foreground {
		me.waitDone(false)
		tcsetpgrp(pgrpid) // oreshellをforegroundにする
	} else {
		go me.waitDone(true)
	}
	return nil
}

func (me *Job) start() (err error) {

	me.processesWg.Add(len(me.processes))

	for _, p := range me.processes {
		log.Logger.Printf("Process Start %+v\n", p)
		err = p.Start(me.foreground)
		if err != nil {
			return err
		}
	}
	return nil
}

func (me *Job) waitDone(print bool) {
	// 起動したプログラムが終了するまで待つ
	me.processesWg.Wait()

	if print {
		log.Logger.Printf("background job done. \n")
		JobStatusNotify <- fmt.Sprintf("Done. %s\n", me.toJobString())
	} else {
		log.Logger.Printf("foreground job done. \n")
	}
}

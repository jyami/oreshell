package job

import (
	"fmt"
	"oreshell/ast"
	"oreshell/log"
	"oreshell/process"
	"strings"
	"sync"
)

type Job struct {
	processes   []*process.Process
	foreground  bool
	processesWg sync.WaitGroup
	id          int
}

func NewJob(pipelineSequence *ast.PipelineSequence) (job *Job, err error) {

	job = &Job{
		foreground: pipelineSequence.Foreground,
		id:         -1,
	}

	Jobs.registerJob(job)

	for _, sc := range pipelineSequence.SimpleCommands {
		p, err := process.NewProcess(sc, &job.processesWg)
		if err != nil {
			return nil, err
		}
		log.Logger.Printf("Process New %+v\n", p)

		job.processes = append(job.processes, p)
	}
	log.Logger.Printf("job : %s", job.toJobString())
	log.Logger.Printf("processes num %d\n", job.processNum())

	for i, p := range job.processes {
		//プロセス数が1つの場合はパイプ設定対象外
		//プロセスが複数でも末尾のプロセスはパイプ設定対象外。
		if i+1 < job.processNum() {
			log.Logger.Printf("pipe %+v\n", p)
			err = p.PipeWithNext(job.processes[i+1])
			if err != nil {
				return nil, err
			}
		}
	}

	return job, nil
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
		s = fmt.Sprintf("[%d] %s &", me.id, s)
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
		JobStatusNotify <- fmt.Sprintf("%s %s\n", me.processes[me.processNum()-1].Status(), me.toJobString())
	} else {
		log.Logger.Printf("foreground job done. \n")
	}
	Jobs.unregisterJob(me)
}

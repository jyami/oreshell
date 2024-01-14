package job

import (
	"oreshell/ast"
	"oreshell/log"
	"oreshell/process"
)

var JobStatusNotify = make(chan string, 10)

func NewJob(pipelineSequence *ast.PipelineSequence) (me *Job, err error) {

	me = &Job{
		foreground: pipelineSequence.Foreground,
	}

	for _, sc := range pipelineSequence.SimpleCommands {
		p, err := process.NewProcess(sc, &me.processesWg)
		if err != nil {
			return nil, err
		}
		log.Logger.Printf("Process New %+v\n", p)

		me.processes = append(me.processes, p)
	}
	log.Logger.Printf("job : %s", me.toJobString())
	log.Logger.Printf("processes num %d\n", me.processNum())

	for i, p := range me.processes {
		//プロセス数が1つの場合はパイプ設定対象外
		//プロセスが複数でも末尾のプロセスはパイプ設定対象外。
		if i+1 < me.processNum() {
			log.Logger.Printf("pipe %+v\n", p)
			err = p.PipeWithNext(me.processes[i+1])
			if err != nil {
				return nil, err
			}
		}
	}

	return me, nil
}

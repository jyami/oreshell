package job

import (
	"container/list"
	"oreshell/log"
	"sync"
)

var JobStatusNotify = make(chan string, 10)

type typeJobs struct {
	mu    sync.Mutex
	items *list.List
}

var Jobs = typeJobs{items: list.New()}

func (me *typeJobs) registerJob(job *Job) {
	log.Logger.Printf("start jobs register job %s\n", job.toJobString())
	lastId := -1

	me.mu.Lock()
	lastElm := me.items.Back()
	if lastElm == nil {
		lastId = 0
	} else {
		lastId = (lastElm.Value.(*Job)).id
	}
	job.id = lastId + 1
	me.items.PushBack(job)
	me.mu.Unlock()

	log.Logger.Printf("end jobs register job %s\n", job.toJobString())
}

func (me *typeJobs) unregisterJob(job *Job) {
	log.Logger.Printf("start jobs unregister job %s\n", job.toJobString())

	me.mu.Lock()
	elm := me.items.Front()
	for elm != nil && elm.Value.(*Job).id != job.id {
		elm = elm.Next()
	}
	if elm == nil {
		// ありえない
		log.Logger.Fatalf("jobs unregister ありえない\n")
	} else {
		me.items.Remove(elm)
	}
	me.mu.Unlock()

	log.Logger.Printf("end jobs unregister job %s\n", job.toJobString())
}

func (me *typeJobs) ListJobString() (a []string) {
	elm := me.items.Front()
	for elm != nil {
		a = append(a, elm.Value.(*Job).toJobString())
		elm = elm.Next()
	}
	return a
}

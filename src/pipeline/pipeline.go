package pipeline

import (
	"sync"
)

type Crawler interface {
	Crawl()
}

//파이프라인은 일처리를 분산시킴.
type Pipeline struct {
	Request chan Crawler  //요청임. r.Crawl()을 할것. 여기서 r은 crawl을 실행할 타입임.따라서 pipeline을 통해 목표하는 대상을 타겟으로 crawl할 수 있음.
	Done    chan struct{} //모두 끝났으면 값이들어감
	Wg      *sync.WaitGroup
}

//새로운 파이프라인생성
func NewPipeline() *Pipeline {
	return &Pipeline{
		Request: make(chan Crawler),
		Done:    make(chan struct{}),
		Wg:      new(sync.WaitGroup),
	}
}

//worker가 request가 오는지 안오는지 계속감시하다 request가 들어오면 crawl실행
func (p *Pipeline) Worker() {
	for r := range p.Request {
		select {
		case <-p.Done: //체널 닫히면 일 고만해라
			return
		default:

			r.Crawl()
		}

	}
}

//파이프라인 가동. 10개의 통로가 있음. cpu능력이 되는한 같은 일을 10배빠른속도로 처리할 수 있다는 거임.
func (p *Pipeline) Run() {
	const numWorkers = 10
	p.Wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			p.Worker()  //worker 끌날때까지 계속 일하소~
			p.Wg.Done() //끝나면 시마이
		}()
	}
	go func() {
		p.Wg.Wait()
	}()
}

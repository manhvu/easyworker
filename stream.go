package easyworker

import (
	"errors"
	"log"
)

/*
Store options and runtime data for stream processing.
Also, struct provides interface for control and processing task.
*/
type EasyStream struct {
	id int

	// config input by user.
	config Config

	// inputs channel.
	inputCh chan []any

	// output channel.
	outputCh chan any

	// cmd channel for supervisor.
	cmdCh chan int

	// store runtime workers.
	workerList map[int]*worker
}

/*
Make new EasyStream.
Config is made before make new EasyTask.

config: instance of Config.
taskCh: channel EasyStream will wait & get task.
resultCh: channel EastyStream will send out result of task.

Example:

	task,_ := NewStream(config)
*/
func NewStream(config Config, taskCh chan []any, resultCh chan any) (ret EasyStream, err error) {
	// auto incremental number, get supervisor's id/
	taskLastId++

	ret = EasyStream{
		id:         taskLastId,
		config:     config,
		inputCh:    taskCh,
		outputCh:   resultCh,
		workerList: make(map[int]*worker, config.worker),
	}

	return
}

/*
Run func to process stream continuously.

Example:

	easyStream.Run()
*/
func (p *EasyStream) Run() (retErr error) {
	// use for send function's params to worker.
	inputCh := make(chan msg, p.config.worker)

	// use for get result from worker.
	resultCh := make(chan msg, p.config.worker)

	p.cmdCh = make(chan int)

	// Start workers
	for i := 0; i < p.config.worker; i++ {
		opt := &worker{
			id:         int64(i),
			fun:        p.config.fun,
			cmd:        make(chan msg),
			resultCh:   resultCh,
			inputCh:    inputCh,
			retryTimes: p.config.retry,
		}
		p.workerList[i] = opt

		go opt.run()
	}

	// Send data to worker
	go func() {
		for {
			params := <-p.inputCh
			if printLog {
				log.Println("stream received new params: ", params)
			}
			inputCh <- msg{id: iSTREAM, msgType: iTASK, data: params}
		}
	}()

	// receive result from worker
	go func() {
		for {
			result := <-resultCh
			switch result.msgType {
			case iSUCCESS: // task done
				p.outputCh <- result.data
			case iERROR: // task failed
				if printLog {
					log.Println("stream task", result.id, " is failed, error:", result.data)
				}
				// send error to outside.
				p.outputCh <- result.data
			case iFATAL_ERROR: // worker panic
				if printLog {
					log.Println(result.id, "worker (stream) is fatal error")
				}
			case iQUIT: // worker quited
				if printLog {
					log.Println(result.id, " exited (stream)")
				}
			}
		}
	}()

	// send signal to worker to stop.
	go func() {
		for {
			cmd := <-p.cmdCh
			switch cmd {
			case iQUIT:
				for i, w := range p.workerList {
					w.cmd <- msg{msgType: iQUIT}
					delete(p.workerList, i)
				}
			}
		}
	}()

	return
}

/*
Stop all workers in stream.
Time to stop depend time user function return.
*/
func (p *EasyStream) Stop() error {
	if p.cmdCh != nil {
		p.cmdCh <- iQUIT
		return nil
	} else {
		return errors.New("EasyWorker isn't sart or wrong task's type")
	}
}

package easyworker

import (
	"errors"
	"log"
)

/*
Store options and runtime data for task processing.
Also, struct provides interface for control and processing task.
*/
type EasyTask struct {
	id int

	// config input by user.
	config Config

	// task for worker. It's slice of slice of params.
	inputs [][]any

	// store runtime workers.
	workerList map[int]*worker
}

/*
Make new EasyTask.
Config is made before make new EasyTask.

Example:

	task,_ := NewTask(config)
*/
func NewTask(config Config) (ret EasyTask, err error) {
	// auto incremental number.
	taskLastId++

	ret = EasyTask{
		id:         taskLastId,
		config:     config,
		inputs:     make([][]any, 0),
		workerList: make(map[int]*worker, config.worker),
	}

	return
}

/*
Uses for adding tasks for EasyTask.

Example:

	workers.AddParams(1, "user")
	workers.AddParams(2, "user")
	workers.AddParams(1000, "admin")
*/
func (p *EasyTask) AddTask(i ...any) {
	params := make([]any, 0)
	params = append(params, i...)

	p.inputs = append(p.inputs, params)
}

/*
Run func with existed task or waiting a new task.

Example:

	easyTask.Run()
*/
func (p *EasyTask) Run() (ret []any, retErr error) {
	ret = make([]any, 0)

	if len(p.inputs) < 1 {
		retErr = errors.New("need params to run")
		return
	}

	// use for send function's params to worker.
	inputCh := make(chan msg, p.config.worker)

	// use for get result from worker.
	resultCh := make(chan msg, p.config.worker)

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
		for index, params := range p.inputs {
			inputCh <- msg{id: index, msgType: iTASK, data: params}
		}
	}()

	resultMap := map[int]any{}

	// receive result from worker
	for {
		result := <-resultCh
		switch result.msgType {
		case iSUCCESS: // task done
			resultMap[result.id] = result.data
		case iERROR: // task failed
			if printLog {
				log.Println("task", result.id, " is failed, error:", result.data)
			}
			resultMap[result.id] = result.data
		case iFATAL_ERROR: // worker panic
			if printLog {
				log.Println(result.id, "worker is fatal error")
			}
		case iQUIT: // worker quited\
			if printLog {
				log.Println(result.id, " exited")
			}
		}

		if len(resultMap) == len(p.inputs) {
			break
		}
	}

	// send signal to worker to stop.
	go func() {
		for _, w := range p.workerList {
			w.cmd <- msg{msgType: iQUIT}
		}
	}()

	ret = make([]any, len(resultMap))

	for k, v := range resultMap {

		ret[k] = v
	}

	return
}

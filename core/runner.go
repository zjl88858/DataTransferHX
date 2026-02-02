package core

import (
	"log"

	"github.com/robfig/cron/v3"

	"filetransferhx/config"
)

type Runner struct {
	Config          *config.Config
	TransferManager *TransferManager
	Cron            *cron.Cron
}

func NewRunner(cfg *config.Config, tm *TransferManager) *Runner {
	return &Runner{
		Config:          cfg,
		TransferManager: tm,
		Cron:            cron.New(),
	}
}

func (r *Runner) Start() {
	for _, task := range r.Config.Tasks {
		task := task // capture loop variable
		_, err := r.Cron.AddFunc(task.Cron, func() {
			err := r.TransferManager.RunTask(task)
			if err != nil {
				log.Printf("Task %s failed: %v", task.Name, err)
			}
		})
		if err != nil {
			log.Printf("Failed to schedule task %s: %v", task.Name, err)
		} else {
			log.Printf("Scheduled task %s with cron %s", task.Name, task.Cron)
		}

		// Run immediately in background
		go func(t config.Task) {
			log.Printf("Executing immediate run for task: %s", t.Name)
			if err := r.TransferManager.RunTask(t); err != nil {
				log.Printf("Immediate run of task %s failed: %v", t.Name, err)
			}
		}(task)
	}
	r.Cron.Start()
}

func (r *Runner) Stop() {
	r.Cron.Stop()
}

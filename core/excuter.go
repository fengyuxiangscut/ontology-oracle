/*
 * Copyright (C) 2018 The ontology Authors
 * This file is part of The ontology library.
 *
 * The ontology is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The ontology is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The ontology.  If not, see <http://www.gnu.org/licenses/>.
 */

package core

import (
	"time"

	"github.com/ontio/ontology-oracle/log"
	"github.com/ontio/ontology-oracle/models"
	"github.com/ontio/ontology-oracle/runners"
)

func (app *OracleApplication) RunJob(job *models.JobSpec) {
	jobRun := job.NewRun()
	t, _ := time.Parse("2006-01-02 15:04:05", jobRun.Scheduler.Params)
	if t.After(time.Now()) {
		return
	}
	jobRun = app.executeRun(jobRun)
	if jobRun.Status == models.RunStatusErrored {
		log.Errorf("Current job run execution error: %v", jobRun.Result.ErrorMessage)

		err := app.sendDataToContract(jobRun)
		if err != nil {
			log.Errorf("send error data to contract error: %v", err.Error())
			return
		} else {
			log.Infof("send error data to contract success, Job ID is: %v", jobRun.JobID)
		}
	}
	if jobRun.Status == models.RunStatusCompleted {
		log.Infof("Finished current job run execution: %v, Job result is: %v", jobRun.ID, jobRun.Result)

		err := app.sendDataToContract(jobRun)
		if err != nil {
			log.Errorf("send success data to contract error: %v", err.Error())
			return
		} else {
			log.Infof("send success data to contract success, Job ID is: %v", jobRun.JobID)
		}
	}

	err := app.Store.Put([]byte(jobRun.JobID), job.Request, nil)
	if err != nil {
		log.Errorf("put request into db error : %v", err)
	}
}

func (app *OracleApplication) executeRun(jobRun models.JobRun) models.JobRun {
	jobRun.Status = models.RunStatusInProgress

	log.Infof("Starting job run: %v, JobID is: %v", jobRun.ID, jobRun.JobID)
	latestRun := jobRun.TaskRuns[0]

	for i, taskRun := range jobRun.TaskRuns {

		log.Debugf("Starting task run: %v", taskRun.ID)
		latestRun = markCompleted(startTask(taskRun, latestRun.Result))
		jobRun.TaskRuns[i] = latestRun
		if latestRun.Result.Status == models.RunStatusErrored {
			break
		}
		log.Debugf("Finish task run: %v", taskRun.ID)

	}

	jobRun = jobRun.ApplyResult(latestRun.Result)
	return jobRun
}

func markCompleted(tr models.TaskRun) models.TaskRun {
	if tr.Status.Runnable() {
		return tr.MarkCompleted()
	}
	return tr
}

func startTask(taskRun models.TaskRun, input models.RunResult) models.TaskRun {

	taskRun.Status = models.RunStatusInProgress
	runner, err := runners.For(taskRun.Task)

	if err != nil {
		log.Errorf("create runners error: %v", err)
		rr := taskRun.Result.WithError(err)
		return taskRun.ApplyResult(rr)
	}

	return taskRun.ApplyResult(runner.Perform(input))
}

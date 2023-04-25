// Copyright 2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqle

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/xiazemin/go-mysql-server/sql"
)

// ProcessList is a structure that keeps track of all the processes and their
// status.
type ProcessList struct {
	mu         sync.RWMutex
	procs      map[uint32]*sql.Process
	byQueryPid map[uint64]uint32
}

// NewProcessList creates a new process list.
func NewProcessList() *ProcessList {
	return &ProcessList{
		procs:      make(map[uint32]*sql.Process),
		byQueryPid: make(map[uint64]uint32),
	}
}

// Processes returns the list of current running processes.
func (pl *ProcessList) Processes() []sql.Process {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	var result = make([]sql.Process, 0, len(pl.procs))

	for _, proc := range pl.procs {
		p := *proc
		var progress = make(map[string]sql.TableProgress, len(p.Progress))
		for n, p := range p.Progress {
			progress[n] = p
		}
		result = append(result, p)
	}

	return result
}

func (pl *ProcessList) AddConnection(id uint32, addr string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.procs[id] = &sql.Process{
		Connection: id,
		Command:    sql.ProcessCommandConnect,
		Host:       addr,
		User:       "unauthenticated user",
		StartedAt:  time.Now(),
	}
}

func (pl *ProcessList) ConnectionReady(sess sql.Session) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.procs[sess.ID()] = &sql.Process{
		Connection: sess.ID(),
		Command:    sql.ProcessCommandSleep,
		Host:       sess.Client().Address,
		User:       sess.Client().User,
		StartedAt:  time.Now(),
	}
}

func (pl *ProcessList) RemoveConnection(connID uint32) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	p := pl.procs[connID]
	if p != nil {
		if p.Kill != nil {
			p.Kill()
		}
		delete(pl.byQueryPid, p.QueryPid)
		delete(pl.procs, connID)
	}
}

func (pl *ProcessList) BeginQuery(
	ctx *sql.Context,
	query string,
) (*sql.Context, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	id := ctx.Session.ID()
	pid := ctx.Pid()
	p := pl.procs[id]
	if p == nil {
		return nil, errors.New("internal error: connection not registered with process list")
	}
	if _, ok := pl.byQueryPid[pid]; ok {
		return nil, sql.ErrPidAlreadyUsed.New(pid)
	}

	newCtx, cancel := context.WithCancel(ctx)
	ctx = ctx.WithContext(newCtx)

	p.Command = sql.ProcessCommandQuery
	p.Query = query
	p.QueryPid = pid
	p.StartedAt = time.Now()
	p.Kill = cancel
	p.Progress = make(map[string]sql.TableProgress)

	pl.byQueryPid[ctx.Pid()] = ctx.Session.ID()

	return ctx, nil
}

func (pl *ProcessList) EndQuery(ctx *sql.Context) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	id := ctx.Session.ID()
	pid := ctx.Pid()
	delete(pl.byQueryPid, pid)
	p := pl.procs[id]
	if p != nil && p.QueryPid == pid {
		p.Command = sql.ProcessCommandSleep
		p.Query = ""
		p.StartedAt = time.Now()
		p.Kill()
		p.Kill = nil
		p.QueryPid = 0
		p.Progress = nil
	}
}

// UpdateTableProgress updates the progress of the table with the given name for the
// process with the given pid.
func (pl *ProcessList) UpdateTableProgress(pid uint64, name string, delta int64) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	id, ok := pl.byQueryPid[pid]
	if !ok {
		return
	}
	p, ok := pl.procs[id]
	if !ok {
		return
	}

	progress, ok := p.Progress[name]
	if !ok {
		progress = sql.NewTableProgress(name, -1)
	}

	progress.Done += delta
	p.Progress[name] = progress
}

// UpdatePartitionProgress updates the progress of the table partition with the
// given name for the process with the given pid.
func (pl *ProcessList) UpdatePartitionProgress(pid uint64, tableName, partitionName string, delta int64) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	id, ok := pl.byQueryPid[pid]
	if !ok {
		return
	}
	p, ok := pl.procs[id]
	if !ok {
		return
	}

	tablePg, ok := p.Progress[tableName]
	if !ok {
		return
	}

	partitionPg, ok := tablePg.PartitionsProgress[partitionName]
	if !ok {
		partitionPg = sql.PartitionProgress{Progress: sql.Progress{Name: partitionName, Total: -1}}
	}

	partitionPg.Done += delta
	tablePg.PartitionsProgress[partitionName] = partitionPg
}

// AddTableProgress adds a new item to track progress from to the process with
// the given pid. If the pid does not exist, it will do nothing.
func (pl *ProcessList) AddTableProgress(pid uint64, name string, total int64) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	id, ok := pl.byQueryPid[pid]
	if !ok {
		return
	}
	p, ok := pl.procs[id]
	if !ok {
		return
	}

	if pg, ok := p.Progress[name]; ok {
		pg.Total = total
		p.Progress[name] = pg
	} else {
		p.Progress[name] = sql.NewTableProgress(name, total)
	}
}

// AddPartitionProgress adds a new item to track progress from to the process with
// the given pid. If the pid or the table does not exist, it will do nothing.
func (pl *ProcessList) AddPartitionProgress(pid uint64, tableName, partitionName string, total int64) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	id, ok := pl.byQueryPid[pid]
	if !ok {
		return
	}
	p, ok := pl.procs[id]
	if !ok {
		return
	}

	tablePg, ok := p.Progress[tableName]
	if !ok {
		return
	}

	if pg, ok := tablePg.PartitionsProgress[partitionName]; ok {
		pg.Total = total
		tablePg.PartitionsProgress[partitionName] = pg
	} else {
		tablePg.PartitionsProgress[partitionName] =
			sql.PartitionProgress{Progress: sql.Progress{Name: partitionName, Total: total}}
	}
}

// RemoveTableProgress removes an existing item tracking progress from the
// process with the given pid, if it exists.
func (pl *ProcessList) RemoveTableProgress(pid uint64, name string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	id, ok := pl.byQueryPid[pid]
	if !ok {
		return
	}
	p, ok := pl.procs[id]
	if !ok {
		return
	}

	delete(p.Progress, name)
}

// RemovePartitionProgress removes an existing item tracking progress from the
// process with the given pid, if it exists.
func (pl *ProcessList) RemovePartitionProgress(pid uint64, tableName, partitionName string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	id, ok := pl.byQueryPid[pid]
	if !ok {
		return
	}
	p, ok := pl.procs[id]
	if !ok {
		return
	}

	tablePg, ok := p.Progress[tableName]
	if !ok {
		return
	}

	delete(tablePg.PartitionsProgress, partitionName)
}

// Kill terminates all queries for a given connection id.
func (pl *ProcessList) Kill(connID uint32) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	p := pl.procs[connID]
	if p != nil && p.Kill != nil {
		logrus.Infof("kill query: pid %d", p.QueryPid)
		p.Kill()
	}
}

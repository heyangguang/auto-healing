package logger

import (
	"sync"

	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger *zap.Logger
	sugar  *zap.SugaredLogger

	categoryWriters = make(map[Category]*lumberjack.Logger)
	categoryMu      sync.RWMutex
	logDir          = "logs"
)

type Category string

const (
	CatAPI   Category = "API"
	CatSched Category = "SCHED"
	CatExec  Category = "EXEC"
	CatSync  Category = "SYNC"
	CatAuth  Category = "AUTH"
)

const (
	SubAPIHeal   = "HEAL"
	SubAPIPlugin = "PLUGIN"
	SubAPIGit    = "GIT"
	SubAPIExec   = "EXEC"
	SubAPIAuth   = "AUTH"
	SubAPIUser   = "USER"
	SubAPINotify = "NOTIFY"

	SubSchedHeal = "HEAL"
	SubSchedSync = "SYNC"
	SubSchedGit  = "GIT"
	SubSchedTask = "TASK"

	SubExecFlow    = "FLOW"
	SubExecNode    = "NODE"
	SubExecAnsible = "ANSIBLE"
	SubExecTask    = "TASK"

	SubSyncPlugin = "PLUGIN"
	SubSyncGit    = "GIT"

	SubAuthSecret = "SECRET"
	SubAuthLogin  = "LOGIN"
	SubAuthToken  = "TOKEN"
)

var categoryToFile = map[Category]string{
	CatAPI:   "api.log",
	CatSched: "scheduler.log",
	CatExec:  "execution.log",
	CatSync:  "sync.log",
	CatAuth:  "auth.log",
}

type CategoryLogger struct {
	category Category
	sub      string
}

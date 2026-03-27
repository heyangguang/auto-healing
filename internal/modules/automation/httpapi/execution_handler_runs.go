package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListAllRuns 获取所有执行记录列表
func (h *ExecutionHandler) ListAllRuns(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	runs, total, err := h.service.ListAllRuns(c.Request.Context(), buildRunListOptions(c, page, pageSize))
	if err != nil {
		respondInternalError(c, "EXEC", "获取执行记录列表失败", err)
		return
	}
	response.List(c, runs, total, page, pageSize)
}

// GetRun 获取执行记录详情
func (h *ExecutionHandler) GetRun(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}
	run, err := h.service.GetRun(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}
	response.Success(c, run)
}

// GetRunLogs 获取执行日志
func (h *ExecutionHandler) GetRunLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}
	logs, err := h.service.GetRunLogs(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "EXEC", "获取执行日志失败", err)
		return
	}
	response.Success(c, logs)
}

// CancelRun 取消执行
func (h *ExecutionHandler) CancelRun(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}
	if err := h.service.CancelRun(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Message(c, "执行已取消")
}

// StreamLogs SSE 实时日志流
func (h *ExecutionHandler) StreamLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}
	if _, err := h.service.GetRun(c.Request.Context(), id); err != nil {
		writeExecutionStreamError(c, "执行记录不存在")
		return
	}

	prepareExecutionLogStream(c)
	lastSeq := 0
	ctx := c.Request.Context()
	c.Stream(func(w io.Writer) bool {
		if ctx.Err() != nil {
			return false
		}
		lastSeq = h.streamNewLogs(ctx, w, id, lastSeq)
		return !h.streamTerminalState(ctx, w, id)
	})
}

func prepareExecutionLogStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	fmt.Fprintf(c.Writer, ": ping\n\n")
	c.Writer.Flush()
}

func writeExecutionStreamError(c *gin.Context, message string) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprintf(c.Writer, "event: error\ndata: {\"message\":\"%s\"}\n\n", message)
	c.Writer.Flush()
}

func (h *ExecutionHandler) streamNewLogs(ctx context.Context, w io.Writer, runID uuid.UUID, lastSeq int) int {
	logs, _ := h.service.GetRunLogs(ctx, runID)
	for _, log := range logs {
		if log.Sequence <= lastSeq {
			continue
		}
		data, _ := json.Marshal(log)
		fmt.Fprintf(w, "event: log\ndata: %s\n\n", string(data))
		flushExecutionWriter(w)
		lastSeq = log.Sequence
	}
	return lastSeq
}

func (h *ExecutionHandler) streamTerminalState(ctx context.Context, w io.Writer, runID uuid.UUID) bool {
	run, _ := h.service.GetRun(ctx, runID)
	if run == nil || !isTerminalRunStatus(run.Status) {
		time.Sleep(200 * time.Millisecond)
		return false
	}

	doneData, _ := json.Marshal(map[string]any{
		"status":    run.Status,
		"exit_code": run.ExitCode,
		"stats":     run.Stats,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", string(doneData))
	flushExecutionWriter(w)
	return true
}

func isTerminalRunStatus(status string) bool {
	switch status {
	case "success", "failed", "cancelled", "partial":
		return true
	default:
		return false
	}
}

func flushExecutionWriter(w io.Writer) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

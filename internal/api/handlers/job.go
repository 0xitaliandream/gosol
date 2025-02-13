package handlers

// import (
// 	"net/http"

// 	"gosol/internal/db"
// 	"gosol/internal/worker"

// 	"github.com/gin-gonic/gin"
// )

// type JobHandler struct {
// 	db     *db.Database
// 	worker *worker.Processor
// }

// func NewJobHandler(database *db.Database, processor *worker.Processor) *JobHandler {
// 	return &JobHandler{
// 		db:     database,
// 		worker: processor,
// 	}
// }

// func (h *JobHandler) List(c *gin.Context) {
// 	status := c.DefaultQuery("status", "")
// 	jobs, err := h.db.ListJobs(c.Request.Context(), status)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, jobs)
// }

// func (h *JobHandler) Get(c *gin.Context) {
// 	id := c.Param("id")
// 	job, err := h.db.GetJob(c.Request.Context(), id)
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, job)
// }

// func (h *JobHandler) Retry(c *gin.Context) {
// 	id := c.Param("id")

// 	job, err := h.db.GetJob(c.Request.Context(), id)
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
// 		return
// 	}

// 	err = h.db.UpdateJobStatus(c.Request.Context(), id, "pending")
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	err = h.worker.EnqueueJob(c.Request.Context(), job)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"message": "Job requeued successfully"})
// }

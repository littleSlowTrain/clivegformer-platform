package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	analysisv1 "github.com/clivegformer/platform/contracts/gen/analysis/v1"
	filev1 "github.com/clivegformer/platform/contracts/gen/file/v1"
	storagev1 "github.com/clivegformer/platform/contracts/gen/storage/v1"
	userv1 "github.com/clivegformer/platform/contracts/gen/user/v1"
	"github.com/clivegformer/platform/gin_web/middlewares"
	"github.com/clivegformer/platform/gin_web/utils"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	users        userv1.UserServiceClient
	files        filev1.FileServiceClient
	storage      storagev1.StorageServiceClient
	analysis     analysisv1.AnalysisServiceClient
	ticketSecret string
}

var fileRegionPattern = regexp.MustCompile(`^[0-9]{1,32}$`)

func New(users userv1.UserServiceClient, files filev1.FileServiceClient, storage storagev1.StorageServiceClient, analysis analysisv1.AnalysisServiceClient, secret string) *Handler {
	return &Handler{users: users, files: files, storage: storage, analysis: analysis, ticketSecret: secret}
}

func (h *Handler) Register(c *gin.Context) {
	var req userv1.RegisterRequest
	if !bind(c, &req) {
		return
	}
	resp, err := h.users.Register(c, &req)
	respond(c, resp, err)
}
func (h *Handler) Login(c *gin.Context) {
	var req userv1.LoginRequest
	if !bind(c, &req) {
		return
	}
	resp, err := h.users.Login(c, &req)
	respond(c, resp, err)
}

type initiateRequest struct {
	FileHash string `json:"file_hash" binding:"required,len=64"`
	Filename string `json:"filename" binding:"required"`
	Folder   string `json:"folder"`
	FileSize int64  `json:"file_size" binding:"required,gt=0"`
}

func (h *Handler) InitiateUpload(c *gin.Context) {
	var req initiateRequest
	if !bind(c, &req) {
		return
	}
	resp, err := h.files.InitiateUpload(c, &filev1.InitiateUploadRequest{UserId: middlewares.UserID(c), FileHash: req.FileHash, Filename: req.Filename, Folder: req.Folder, FileSize: req.FileSize})
	respond(c, resp, err)
}
func (h *Handler) GetUpload(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.files.GetUpload(c, &filev1.GetUploadRequest{UserId: middlewares.UserID(c), UploadId: id})
	respond(c, resp, err)
}
func (h *Handler) UploadPart(c *gin.Context) {
	uploadID, ok := pathID(c)
	if !ok {
		return
	}
	part64, err := strconv.ParseInt(c.Param("part"), 10, 32)
	if err != nil || part64 < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分片编号不正确"})
		return
	}
	size := c.Request.ContentLength
	if size <= 0 {
		c.JSON(http.StatusLengthRequired, gin.H{"error": "必须提供Content-Length"})
		return
	}
	stream, err := h.storage.UploadPart(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	meta := &storagev1.UploadPartMeta{UserId: middlewares.UserID(c), UploadId: uploadID, PartNumber: int32(part64), Size: size, ChunkSha256: c.GetHeader("X-Chunk-SHA256")}
	buffer := make([]byte, 64<<10)
	first := true
	for {
		n, readErr := c.Request.Body.Read(buffer)
		if n > 0 {
			frame := &storagev1.UploadPartRequest{Data: append([]byte(nil), buffer[:n]...)}
			if first {
				frame.Meta = meta
				first = false
			}
			if err := stream.Send(frame); err != nil {
				respond(c, nil, err)
				return
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "读取分片失败"})
			return
		}
	}
	if first {
		_ = stream.Send(&storagev1.UploadPartRequest{Meta: meta})
	}
	resp, err := stream.CloseAndRecv()
	respond(c, resp, err)
}
func (h *Handler) CompleteUpload(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.files.CompleteUpload(c, &filev1.CompleteUploadRequest{UserId: middlewares.UserID(c), UploadId: id})
	if err == nil {
		c.JSON(http.StatusAccepted, resp)
		return
	}
	respond(c, nil, err)
}
func (h *Handler) PauseUpload(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.files.PauseUpload(c, &filev1.UploadActionRequest{UserId: middlewares.UserID(c), UploadId: id})
	respond(c, resp, err)
}
func (h *Handler) ResumeUpload(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.files.ResumeUpload(c, &filev1.UploadActionRequest{UserId: middlewares.UserID(c), UploadId: id})
	respond(c, resp, err)
}
func (h *Handler) HeartbeatUpload(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.files.HeartbeatUpload(c, &filev1.UploadActionRequest{UserId: middlewares.UserID(c), UploadId: id})
	respond(c, resp, err)
}
func (h *Handler) CancelUpload(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.files.CancelUpload(c, &filev1.UploadActionRequest{UserId: middlewares.UserID(c), UploadId: id})
	respond(c, resp, err)
}

func (h *Handler) ListFiles(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	size, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	scope := strings.ToLower(strings.TrimSpace(c.DefaultQuery("scope", "mine")))
	if scope != "mine" && scope != "team" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope必须是mine或team"})
		return
	}
	region := strings.TrimSpace(c.Query("region"))
	if region != "" && !fileRegionPattern.MatchString(region) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "region必须是1至32位数字"})
		return
	}
	var year uint64
	if rawYear := strings.TrimSpace(c.Query("year")); rawYear != "" {
		if len(rawYear) != 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "year必须是四位年份"})
			return
		}
		parsed, parseErr := strconv.ParseUint(rawYear, 10, 32)
		if parseErr != nil || parsed < 1000 || parsed > 9999 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "year必须是四位年份"})
			return
		}
		year = parsed
	}
	resp, err := h.files.ListFiles(c, &filev1.ListFilesRequest{UserId: middlewares.UserID(c), Page: int32(page), PageSize: int32(size), Query: c.Query("query"), Folder: c.Query("folder"), Scope: scope, RegionCode: region, DataYear: uint32(year)})
	respond(c, resp, err)
}

func (h *Handler) ListFileFacets(c *gin.Context) {
	scope := strings.ToLower(strings.TrimSpace(c.DefaultQuery("scope", "mine")))
	if scope != "mine" && scope != "team" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope必须是mine或team"})
		return
	}
	resp, err := h.files.ListFileFacets(c, &filev1.ListFileFacetsRequest{UserId: middlewares.UserID(c), Scope: scope})
	respond(c, resp, err)
}
func (h *Handler) GetFile(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.files.GetFile(c, &filev1.GetFileRequest{UserId: middlewares.UserID(c), UserFileId: id})
	respond(c, resp, err)
}
func (h *Handler) DownloadTicket(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.files.GetFile(c, &filev1.GetFileRequest{UserId: middlewares.UserID(c), UserFileId: id}); err != nil {
		respond(c, nil, err)
		return
	}
	ticket, err := utils.SignDownloadTicket(h.ticketSecret, middlewares.UserID(c), id, 5*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建下载凭证失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "url": "/api/v1/download/" + ticket, "expires_in": 300})
}
func (h *Handler) Download(c *gin.Context) {
	ticket, err := utils.VerifyDownloadTicket(h.ticketSecret, c.Param("ticket"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "下载凭证无效或已过期"})
		return
	}
	info, err := h.files.GetFile(c, &filev1.GetFileRequest{UserId: ticket.UserID, UserFileId: ticket.FileID})
	if err != nil {
		respond(c, nil, err)
		return
	}
	start, end, partial, err := utils.ParseRange(c.GetHeader("Range"), info.FileSize)
	if err != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", info.FileSize))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	stream, err := h.storage.Download(c, &storagev1.DownloadRequest{UserId: ticket.UserID, UserFileId: ticket.FileID, RangeStart: start, RangeEnd: end})
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(info.Filename)))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(end-start+1, 10))
	if partial {
		c.Header("Content-Range", utils.ContentRange(start, end, info.FileSize))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}
	for {
		chunk, recvErr := stream.Recv()
		if recvErr == io.EOF {
			return
		}
		if recvErr != nil {
			return
		}
		_, _ = c.Writer.Write(chunk.Data)
		c.Writer.Flush()
	}
}

type analysisRequest struct {
	FileID      uint64 `json:"file_id" binding:"required"`
	Variable    string `json:"variable"`
	RedVariable string `json:"red_variable"`
	NIRVariable string `json:"nir_variable"`
	MaxPoints   int32  `json:"max_points"`
}

func (h *Handler) Variables(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.analysis.ListVariables(c, &analysisv1.VariablesRequest{UserId: middlewares.UserID(c), UserFileId: id})
	respond(c, resp, err)
}
func (h *Handler) NDVI(c *gin.Context)       { h.analyze(c, true) }
func (h *Handler) TimeSeries(c *gin.Context) { h.analyze(c, false) }
func (h *Handler) analyze(c *gin.Context, ndvi bool) {
	var req analysisRequest
	if !bind(c, &req) {
		return
	}
	rpc := &analysisv1.AnalysisRequest{UserId: middlewares.UserID(c), UserFileId: req.FileID, Variable: req.Variable, RedVariable: req.RedVariable, NirVariable: req.NIRVariable, MaxPoints: req.MaxPoints}
	var resp *analysisv1.AnalysisResponse
	var err error
	if ndvi {
		resp, err = h.analysis.AnalyzeNDVI(c, rpc)
	} else {
		resp, err = h.analysis.AnalyzeTimeSeries(c, rpc)
	}
	respond(c, resp, err)
}

func bind(c *gin.Context, value any) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}
func pathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID不正确"})
		return 0, false
	}
	return id, true
}
func respond(c *gin.Context, value any, err error) {
	if err == nil {
		c.JSON(http.StatusOK, value)
		return
	}
	code := status.Code(err)
	httpCode := http.StatusInternalServerError
	switch code {
	case codes.InvalidArgument:
		httpCode = http.StatusBadRequest
	case codes.Unauthenticated:
		httpCode = http.StatusUnauthorized
	case codes.PermissionDenied:
		httpCode = http.StatusForbidden
	case codes.NotFound:
		httpCode = http.StatusNotFound
	case codes.AlreadyExists:
		httpCode = http.StatusConflict
	case codes.FailedPrecondition:
		httpCode = http.StatusPreconditionFailed
	case codes.Unavailable:
		httpCode = http.StatusServiceUnavailable
	}
	c.JSON(httpCode, gin.H{"error": status.Convert(err).Message()})
}

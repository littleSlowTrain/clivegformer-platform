package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	filev1 "github.com/clivegformer/platform/contracts/gen/file/v1"
	storagev1 "github.com/clivegformer/platform/contracts/gen/storage/v1"
	"github.com/clivegformer/platform/file_srv/model"
	"github.com/clivegformer/platform/file_srv/utils"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Server struct {
	filev1.UnimplementedFileServiceServer
	db      *gorm.DB
	redis   *redis.Client
	storage storagev1.StorageServiceClient
	bucket  string
}

func New(db *gorm.DB, cache *redis.Client, storage storagev1.StorageServiceClient, bucket string) *Server {
	return &Server{db: db, redis: cache, storage: storage, bucket: bucket}
}

func (s *Server) InitiateUpload(ctx context.Context, req *filev1.InitiateUploadRequest) (*filev1.InitiateUploadResponse, error) {
	if req.UserId == 0 || req.FileSize <= 0 || !utils.ValidHash(req.FileHash) || strings.TrimSpace(req.Filename) == "" {
		return nil, status.Error(codes.InvalidArgument, "文件参数不正确")
	}
	req.Folder = utils.NormalizeFolder(req.Folder)
	if response, ok, err := s.attachExisting(ctx, req); ok || err != nil {
		return response, err
	}

	lockKey, lockValue := "lock_hash:"+req.FileHash, uuid.NewString()
	locked, err := s.redis.SetNX(ctx, lockKey, lockValue, 30*time.Second).Result()
	if err != nil {
		return nil, status.Error(codes.Unavailable, "上传锁不可用")
	}
	if !locked {
		return &filev1.InitiateUploadResponse{Decision: filev1.UploadDecision_WAIT, Message: "相同文件正在初始化"}, nil
	}
	defer s.redis.Eval(ctx, `if redis.call("get",KEYS[1]) == ARGV[1] then return redis.call("del",KEYS[1]) else return 0 end`, []string{lockKey}, lockValue)

	if response, ok, err := s.attachExisting(ctx, req); ok || err != nil {
		return response, err
	}
	var existing model.UploadSession
	err = s.db.WithContext(ctx).Where("file_hash = ?", req.FileHash).First(&existing).Error
	if err == nil {
		if (existing.Status == model.UploadUploading || existing.Status == model.UploadInit || existing.Status == model.UploadPaused) && existing.FileSize == req.FileSize {
			heartbeat, _ := s.redis.Exists(ctx, fmt.Sprintf("upload_heartbeat:%d", existing.ID)).Result()
			if existing.UserID != req.UserId && heartbeat > 0 {
				_ = s.redis.SAdd(ctx, "upload_waiters:"+req.FileHash, req.UserId).Err()
				return &filev1.InitiateUploadResponse{Decision: filev1.UploadDecision_WAIT, Upload: toUploadInfo(existing, nil), Message: "相同文件正在上传"}, nil
			}
			existing.UserID, existing.Filename, existing.Folder, existing.Status = req.UserId, req.Filename, req.Folder, model.UploadUploading
			if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
				return nil, status.Error(codes.Internal, "接管上传失败")
			}
			_ = s.touchHeartbeat(ctx, existing.ID)
			return &filev1.InitiateUploadResponse{Decision: filev1.UploadDecision_UPLOAD, Upload: toUploadInfo(existing, nil), Message: "已恢复上传会话"}, nil
		}
		if existing.Status != model.UploadFailed && existing.Status != model.UploadCancelled {
			return &filev1.InitiateUploadResponse{Decision: filev1.UploadDecision_WAIT, Message: "相同Hash状态尚未释放"}, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.Internal, "查询上传状态失败")
	}

	mode, chunkSize, chunkCount := utils.UploadPlan(req.FileSize)
	session := model.UploadSession{UserID: req.UserId, FileHash: req.FileHash, Filename: req.Filename, Folder: req.Folder, FileSize: req.FileSize, ChunkSize: chunkSize, ChunkCount: chunkCount, UploadMode: mode, Bucket: s.bucket, StoragePath: utils.ObjectKey(req.FileHash), Status: model.UploadInit}
	if existing.ID != 0 {
		session.ID = existing.ID
		if err := s.db.WithContext(ctx).Save(&session).Error; err != nil {
			return nil, status.Error(codes.Internal, "重建上传会话失败")
		}
	} else if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return nil, status.Error(codes.Internal, "创建上传会话失败")
	}
	begin, err := s.storage.BeginUpload(ctx, &storagev1.BeginUploadRequest{UploadId: session.ID, Bucket: session.Bucket, ObjectKey: session.StoragePath, Mode: int32(mode), ContentType: "application/octet-stream"})
	if err != nil {
		_ = s.db.WithContext(ctx).Model(&session).Update("status", model.UploadFailed).Error
		return nil, status.Error(codes.Unavailable, "对象存储初始化失败")
	}
	session.CephUploadID, session.Status = begin.CephUploadId, model.UploadUploading
	if err := s.db.WithContext(ctx).Save(&session).Error; err != nil {
		return nil, status.Error(codes.Internal, "保存上传会话失败")
	}
	_ = s.touchHeartbeat(ctx, session.ID)
	return &filev1.InitiateUploadResponse{Decision: filev1.UploadDecision_UPLOAD, Upload: toUploadInfo(session, nil)}, nil
}

func (s *Server) attachExisting(ctx context.Context, req *filev1.InitiateUploadRequest) (*filev1.InitiateUploadResponse, bool, error) {
	var object model.FileObject
	err := s.db.WithContext(ctx).Where("file_hash = ? AND status = ?", req.FileHash, model.FileComplete).First(&object).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, status.Error(codes.Internal, "查询文件失败")
	}
	if object.FileSize != req.FileSize {
		return nil, true, status.Error(codes.FailedPrecondition, "Hash对应文件大小不一致")
	}
	metadata := utils.ParseScientificFilename(req.Filename)
	if metadata.Classified && object.RegionCode == nil {
		updates := metadataUpdates(metadata)
		if updateErr := s.db.WithContext(ctx).Model(&object).Updates(updates).Error; updateErr != nil {
			return nil, true, status.Error(codes.Internal, "补充文件科研元数据失败")
		}
		object.RegionCode, object.BlockIndex, object.DataYear = &metadata.RegionCode, &metadata.BlockIndex, &metadata.DataYear
	}
	userFile := model.UserFile{UserID: req.UserId, FileID: object.ID, Filename: req.Filename, Folder: req.Folder}
	_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&userFile).Error
	if userFile.ID == 0 {
		_ = s.db.WithContext(ctx).Where("user_id=? AND file_id=? AND folder=? AND filename=?", req.UserId, object.ID, req.Folder, req.Filename).First(&userFile).Error
	}
	return &filev1.InitiateUploadResponse{Decision: filev1.UploadDecision_INSTANT, File: toFileInfo(userFile, object), Message: "秒传成功"}, true, nil
}

func (s *Server) GetUpload(ctx context.Context, req *filev1.GetUploadRequest) (*filev1.UploadInfo, error) {
	session, err := s.getSession(ctx, req.UserId, req.UploadId)
	if err != nil {
		return nil, err
	}
	parts := s.loadParts(ctx, session)
	return toUploadInfo(session, parts), nil
}

func (s *Server) ResolveUpload(ctx context.Context, req *filev1.GetUploadRequest) (*filev1.AuthorizePartResponse, error) {
	session, err := s.getSession(ctx, req.UserId, req.UploadId)
	if err != nil {
		return nil, err
	}
	return &filev1.AuthorizePartResponse{Bucket: session.Bucket, ObjectKey: session.StoragePath, CephUploadId: session.CephUploadID, Mode: filev1.UploadMode(session.UploadMode), FileHash: session.FileHash, ExpectedSize: session.FileSize}, nil
}

func (s *Server) AuthorizePart(ctx context.Context, req *filev1.AuthorizePartRequest) (*filev1.AuthorizePartResponse, error) {
	session, err := s.getSession(ctx, req.UserId, req.UploadId)
	if err != nil {
		return nil, err
	}
	if session.Status != model.UploadUploading {
		return nil, status.Error(codes.FailedPrecondition, "上传会话不可写")
	}
	if req.PartNumber < 1 || int(req.PartNumber) > session.ChunkCount || req.Size <= 0 || req.Size > int64(session.ChunkSize) {
		return nil, status.Error(codes.InvalidArgument, "分片参数不正确")
	}
	if int(req.PartNumber) < session.ChunkCount && req.Size != int64(session.ChunkSize) {
		return nil, status.Error(codes.InvalidArgument, "非末尾分片大小不正确")
	}
	_ = s.touchHeartbeat(ctx, session.ID)
	return &filev1.AuthorizePartResponse{Bucket: session.Bucket, ObjectKey: session.StoragePath, CephUploadId: session.CephUploadID, Mode: filev1.UploadMode(session.UploadMode), FileHash: session.FileHash, ExpectedSize: req.Size}, nil
}

func (s *Server) RecordPart(ctx context.Context, req *filev1.RecordPartRequest) (*filev1.Empty, error) {
	part := &filev1.UploadedPart{PartNumber: req.PartNumber, Etag: req.Etag, Size: req.Size, Status: "SUCCESS"}
	data, _ := json.Marshal(part)
	if err := s.redis.HSet(ctx, fmt.Sprintf("upload_parts:%d", req.UploadId), req.PartNumber, data).Err(); err != nil {
		return nil, status.Error(codes.Unavailable, "分片状态缓存失败")
	}
	_ = s.touchHeartbeat(ctx, req.UploadId)
	return &filev1.Empty{}, nil
}

func (s *Server) CompleteUpload(ctx context.Context, req *filev1.CompleteUploadRequest) (*filev1.UploadInfo, error) {
	session, err := s.getSession(ctx, req.UserId, req.UploadId)
	if err != nil {
		return nil, err
	}
	if session.Status == model.UploadComplete {
		return toUploadInfo(session, nil), nil
	}
	if session.Status == model.UploadFinalizing {
		return toUploadInfo(session, s.loadParts(ctx, session)), nil
	}
	if session.Status != model.UploadUploading {
		return nil, status.Error(codes.FailedPrecondition, "当前上传状态不能执行完成操作")
	}
	parts := s.loadParts(ctx, session)
	if len(parts) != session.ChunkCount {
		return nil, status.Errorf(codes.FailedPrecondition, "分片不完整：%d/%d", len(parts), session.ChunkCount)
	}
	storageParts := make([]*storagev1.Part, 0, len(parts))
	for _, part := range parts {
		storageParts = append(storageParts, &storagev1.Part{PartNumber: part.PartNumber, Etag: part.Etag, Size: part.Size})
	}
	transition := s.db.WithContext(ctx).Model(&model.UploadSession{}).
		Where("id=? AND status=?", session.ID, model.UploadUploading).
		Update("status", model.UploadFinalizing)
	if transition.Error != nil {
		return nil, status.Error(codes.Internal, "锁定上传完成状态失败")
	}
	if transition.RowsAffected != 1 {
		return nil, status.Error(codes.FailedPrecondition, "上传状态已变化，请刷新后重试")
	}
	session.Status = model.UploadFinalizing
	if _, err := s.storage.CompleteUpload(ctx, &storagev1.CompleteUploadRequest{UserId: req.UserId, UploadId: req.UploadId, Parts: storageParts}); err != nil {
		_ = s.db.WithContext(ctx).Model(&model.UploadSession{}).
			Where("id=? AND status=?", session.ID, model.UploadFinalizing).
			Update("status", model.UploadUploading).Error
		return nil, status.Error(codes.Unavailable, "完成Ceph对象失败")
	}
	return toUploadInfo(session, parts), nil
}

func (s *Server) PauseUpload(ctx context.Context, req *filev1.UploadActionRequest) (*filev1.UploadInfo, error) {
	return s.setStatus(ctx, req, model.UploadPaused)
}
func (s *Server) ResumeUpload(ctx context.Context, req *filev1.UploadActionRequest) (*filev1.UploadInfo, error) {
	info, err := s.setStatus(ctx, req, model.UploadUploading)
	if err == nil {
		_ = s.touchHeartbeat(ctx, req.UploadId)
	}
	return info, err
}
func (s *Server) HeartbeatUpload(ctx context.Context, req *filev1.UploadActionRequest) (*filev1.Empty, error) {
	if _, err := s.getSession(ctx, req.UserId, req.UploadId); err != nil {
		return nil, err
	}
	if err := s.touchHeartbeat(ctx, req.UploadId); err != nil {
		return nil, status.Error(codes.Unavailable, "刷新上传心跳失败")
	}
	return &filev1.Empty{}, nil
}

func (s *Server) CancelUpload(ctx context.Context, req *filev1.UploadActionRequest) (*filev1.Empty, error) {
	session, err := s.getSession(ctx, req.UserId, req.UploadId)
	if err != nil {
		return nil, err
	}
	if session.Status == model.UploadComplete || session.Status == model.UploadFinalizing {
		return nil, status.Error(codes.FailedPrecondition, "文件已完成或正在确认，不能取消")
	}
	if session.Status == model.UploadCancelled {
		return &filev1.Empty{}, nil
	}
	previousStatus := session.Status
	transition := s.db.WithContext(ctx).Model(&model.UploadSession{}).
		Where("id=? AND status IN ?", session.ID, []int{model.UploadInit, model.UploadUploading, model.UploadPaused, model.UploadFailed}).
		Update("status", model.UploadCancelled)
	if transition.Error != nil || transition.RowsAffected != 1 {
		return nil, status.Error(codes.FailedPrecondition, "上传状态已变化，不能取消")
	}
	if _, err := s.storage.AbortUpload(ctx, &storagev1.AbortUploadRequest{UserId: req.UserId, UploadId: req.UploadId}); err != nil {
		_ = s.db.WithContext(ctx).Model(&model.UploadSession{}).
			Where("id=? AND status=?", session.ID, model.UploadCancelled).
			Update("status", previousStatus).Error
		return nil, status.Error(codes.Unavailable, "终止Ceph上传失败")
	}
	_ = s.redis.Del(ctx, fmt.Sprintf("upload_parts:%d", session.ID), fmt.Sprintf("upload_heartbeat:%d", session.ID)).Err()
	return &filev1.Empty{}, nil
}

func (s *Server) ListFiles(ctx context.Context, req *filev1.ListFilesRequest) (*filev1.ListFilesResponse, error) {
	return s.listFiles(ctx, req.UserId, req.Page, req.PageSize, req.Query, req.Folder, req.Scope, req.RegionCode, req.DataYear)
}
func (s *Server) SearchFiles(ctx context.Context, req *filev1.SearchFilesRequest) (*filev1.ListFilesResponse, error) {
	return s.listFiles(ctx, req.UserId, req.Page, req.PageSize, req.Query, "", "mine", "", 0)
}

func (s *Server) listFiles(ctx context.Context, userID uint64, page, pageSize int32, query, folder, scope, regionCode string, dataYear uint32) (*filev1.ListFilesResponse, error) {
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "用户身份无效")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if strings.EqualFold(strings.TrimSpace(scope), "team") {
		return s.listTeamFiles(ctx, userID, page, pageSize, query, regionCode, dataYear)
	}
	buildBase := func() *gorm.DB {
		base := s.db.WithContext(ctx).Table("user_file uf").
			Joins("JOIN file_object fo ON fo.id=uf.file_id").
			Joins("JOIN `user` owner ON owner.id=uf.user_id").
			Where("uf.user_id=? AND fo.status=?", userID, model.FileComplete)
		if query != "" {
			like := "%" + strings.TrimSpace(query) + "%"
			base = base.Where("uf.filename LIKE ? OR fo.file_hash LIKE ?", like, like)
		}
		if folder != "" {
			base = base.Where("uf.folder=?", utils.NormalizeFolder(folder))
		}
		if regionCode != "" {
			base = base.Where("fo.region_code=?", regionCode)
		}
		if dataYear != 0 {
			base = base.Where("fo.data_year=?", dataYear)
		}
		return base
	}
	var total int64
	if err := buildBase().Count(&total).Error; err != nil {
		return nil, status.Error(codes.Internal, "统计文件失败")
	}
	var totalSize int64
	if err := buildBase().Select("COALESCE(SUM(fo.file_size),0)").Scan(&totalSize).Error; err != nil {
		return nil, status.Error(codes.Internal, "统计文件容量失败")
	}
	var rows []fileListRow
	selectSQL := "uf.id,uf.file_id,uf.filename,uf.folder,fo.file_hash,fo.file_size,fo.region_code,fo.block_index,fo.data_year,uf.created_at,uf.user_id owner_user_id,owner.username owner_username," +
		"(SELECT COUNT(*) FROM user_file refs WHERE refs.file_id=uf.file_id) reference_count,TRUE owned_by_me"
	if err := buildBase().Select(selectSQL).Order("uf.created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Scan(&rows).Error; err != nil {
		return nil, status.Error(codes.Internal, "查询文件失败")
	}
	return &filev1.ListFilesResponse{Items: fileRows(rows), Total: total, Page: page, PageSize: pageSize, TotalSize: totalSize}, nil
}

type fileListRow struct {
	ID, FileID, OwnerUserID    uint64
	Filename, Folder, FileHash string
	OwnerUsername              string
	FileSize, ReferenceCount   int64
	OwnedByMe                  bool
	CreatedAt                  time.Time
	RegionCode                 *string
	BlockIndex                 *uint32
	DataYear                   *uint32
}

func fileRows(rows []fileListRow) []*filev1.FileInfo {
	items := make([]*filev1.FileInfo, 0, len(rows))
	for _, row := range rows {
		item := &filev1.FileInfo{Id: row.ID, FileId: row.FileID, Filename: row.Filename, Folder: row.Folder, FileHash: row.FileHash, FileSize: row.FileSize, Status: "COMPLETE", CreatedAt: row.CreatedAt.Unix(), OwnerUserId: row.OwnerUserID, OwnerUsername: row.OwnerUsername, ReferenceCount: row.ReferenceCount, OwnedByMe: row.OwnedByMe}
		applyFileMetadata(item, row.RegionCode, row.BlockIndex, row.DataYear)
		items = append(items, item)
	}
	return items
}

func (s *Server) listTeamFiles(ctx context.Context, userID uint64, page, pageSize int32, query, regionCode string, dataYear uint32) (*filev1.ListFilesResponse, error) {
	buildBase := func() *gorm.DB {
		base := s.db.WithContext(ctx).Table("file_object fo").
			Joins("JOIN user_file uf ON uf.id=(SELECT MIN(first_ref.id) FROM user_file first_ref WHERE first_ref.file_id=fo.id)").
			Joins("JOIN `user` owner ON owner.id=uf.user_id").
			Where("fo.status=?", model.FileComplete)
		if strings.TrimSpace(query) != "" {
			like := "%" + strings.TrimSpace(query) + "%"
			base = base.Where("fo.file_name LIKE ? OR fo.file_hash LIKE ? OR EXISTS (SELECT 1 FROM user_file aliases WHERE aliases.file_id=fo.id AND aliases.filename LIKE ?)", like, like, like)
		}
		if regionCode != "" {
			base = base.Where("fo.region_code=?", regionCode)
		}
		if dataYear != 0 {
			base = base.Where("fo.data_year=?", dataYear)
		}
		return base
	}
	var total, totalSize int64
	if err := buildBase().Count(&total).Error; err != nil {
		return nil, status.Error(codes.Internal, "统计课题组文件失败")
	}
	if err := buildBase().Select("COALESCE(SUM(fo.file_size),0)").Scan(&totalSize).Error; err != nil {
		return nil, status.Error(codes.Internal, "统计课题组容量失败")
	}
	var rows []fileListRow
	selectSQL := "uf.id,fo.id file_id,fo.file_name filename,'/' folder,fo.file_hash,fo.file_size,fo.region_code,fo.block_index,fo.data_year,fo.created_at,uf.user_id owner_user_id,owner.username owner_username," +
		"(SELECT COUNT(DISTINCT refs.user_id) FROM user_file refs WHERE refs.file_id=fo.id) reference_count," +
		"EXISTS(SELECT 1 FROM user_file mine WHERE mine.file_id=fo.id AND mine.user_id=?) owned_by_me"
	if err := buildBase().Select(selectSQL, userID).Order("fo.created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Scan(&rows).Error; err != nil {
		return nil, status.Error(codes.Internal, "查询课题组文件失败")
	}
	return &filev1.ListFilesResponse{Items: fileRows(rows), Total: total, Page: page, PageSize: pageSize, TotalSize: totalSize}, nil
}

func (s *Server) ListFileFacets(ctx context.Context, req *filev1.ListFileFacetsRequest) (*filev1.ListFileFacetsResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.Unauthenticated, "用户身份无效")
	}
	buildBase := func() *gorm.DB {
		base := s.db.WithContext(ctx).Table("file_object fo").Where("fo.status=? AND fo.region_code IS NOT NULL AND fo.data_year IS NOT NULL", model.FileComplete)
		if !strings.EqualFold(strings.TrimSpace(req.Scope), "team") {
			base = base.Joins("JOIN user_file facet_uf ON facet_uf.file_id=fo.id AND facet_uf.user_id=?", req.UserId)
		}
		return base
	}
	var regions []string
	if err := buildBase().Distinct("fo.region_code").Order("fo.region_code ASC").Pluck("fo.region_code", &regions).Error; err != nil {
		return nil, status.Error(codes.Internal, "查询区域筛选项失败")
	}
	var years []uint32
	if err := buildBase().Distinct("fo.data_year").Order("fo.data_year DESC").Pluck("fo.data_year", &years).Error; err != nil {
		return nil, status.Error(codes.Internal, "查询年份筛选项失败")
	}
	return &filev1.ListFileFacetsResponse{Regions: regions, Years: years}, nil
}

func (s *Server) GetFile(ctx context.Context, req *filev1.GetFileRequest) (*filev1.FileInfo, error) {
	uf, fo, err := s.findUserFile(ctx, req.UserId, req.UserFileId)
	if err != nil {
		return nil, err
	}
	info := toFileInfo(uf, fo)
	var owner struct{ Username string }
	_ = s.db.WithContext(ctx).Table("`user`").Select("username").Where("id=?", uf.UserID).Scan(&owner).Error
	var references int64
	_ = s.db.WithContext(ctx).Model(&model.UserFile{}).Where("file_id=?", uf.FileID).Count(&references).Error
	info.OwnerUserId, info.OwnerUsername, info.ReferenceCount, info.OwnedByMe = uf.UserID, owner.Username, references, uf.UserID == req.UserId
	applyFileMetadata(info, fo.RegionCode, fo.BlockIndex, fo.DataYear)
	return info, nil
}
func (s *Server) ResolveDownload(ctx context.Context, req *filev1.ResolveDownloadRequest) (*filev1.ResolvedObject, error) {
	uf, fo, err := s.findUserFile(ctx, req.UserId, req.UserFileId)
	if err != nil {
		return nil, err
	}
	return &filev1.ResolvedObject{Bucket: fo.Bucket, ObjectKey: fo.StoragePath, Filename: uf.Filename, Size: fo.FileSize, FileHash: fo.FileHash}, nil
}

func (s *Server) AcquireAnalysisCache(ctx context.Context, req *filev1.AcquireAnalysisCacheRequest) (*filev1.AcquireAnalysisCacheResponse, error) {
	if req.UserId == 0 || req.UserFileId == 0 || strings.TrimSpace(req.AnalysisType) == "" || strings.TrimSpace(req.CacheVersion) == "" {
		return nil, status.Error(codes.InvalidArgument, "分析缓存参数不完整")
	}
	_, object, err := s.findUserFile(ctx, req.UserId, req.UserFileId)
	if err != nil {
		return nil, err
	}
	var parameters any
	if err := json.Unmarshal([]byte(req.ParametersJson), &parameters); err != nil {
		return nil, status.Error(codes.InvalidArgument, "分析参数不是有效JSON")
	}
	canonical, _ := json.Marshal(parameters)
	analysisType := strings.ToLower(strings.TrimSpace(req.AnalysisType))
	material := fmt.Sprintf("%d\n%s\n%s\n%s\n%s", object.ID, analysisType, canonical, req.CacheVersion, req.Model)
	cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(material)))
	now := time.Now().UTC()

	for attempt := 0; attempt < 3; attempt++ {
		var result model.AnalysisResult
		var response *filev1.AcquireAnalysisCacheResponse
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			lookup := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cache_key=?", cacheKey).First(&result)
			if errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
				token := uuid.NewString()
				lease := now.Add(90 * time.Second)
				result = model.AnalysisResult{FileID: object.ID, AnalysisType: analysisType, ParametersJSON: string(canonical), CacheKey: cacheKey, CacheVersion: req.CacheVersion, Model: req.Model, Status: model.AnalysisProcessing, LeaseToken: token, LeaseExpiresAt: &lease, CreatedByUserID: req.UserId, LastAccessedAt: now}
				if createErr := tx.Create(&result).Error; createErr != nil {
					return createErr
				}
				response = &filev1.AcquireAnalysisCacheResponse{Decision: filev1.AnalysisCacheDecision_CACHE_COMPUTE, Entry: cacheEntry(result), LeaseToken: token}
				return nil
			}
			if lookup.Error != nil {
				return lookup.Error
			}
			if result.Status == model.AnalysisComplete && (result.ExpiresAt == nil || result.ExpiresAt.After(now)) {
				if updateErr := tx.Model(&result).Updates(map[string]any{"hit_count": gorm.Expr("hit_count + 1"), "last_accessed_at": now}).Error; updateErr != nil {
					return updateErr
				}
				response = &filev1.AcquireAnalysisCacheResponse{Decision: filev1.AnalysisCacheDecision_CACHE_HIT, Entry: cacheEntry(result)}
				return nil
			}
			if result.Status == model.AnalysisProcessing && result.LeaseExpiresAt != nil && result.LeaseExpiresAt.After(now) {
				response = &filev1.AcquireAnalysisCacheResponse{Decision: filev1.AnalysisCacheDecision_CACHE_WAIT, Entry: cacheEntry(result), RetryAfterMs: 500}
				return nil
			}
			token := uuid.NewString()
			lease := now.Add(90 * time.Second)
			updates := map[string]any{"status": model.AnalysisProcessing, "lease_token": token, "lease_expires_at": lease, "expires_at": nil, "error_message": "", "result_json": nil, "provider": "", "created_by_user_id": req.UserId, "last_accessed_at": now}
			if updateErr := tx.Model(&result).Updates(updates).Error; updateErr != nil {
				return updateErr
			}
			result.LeaseToken, result.LeaseExpiresAt, result.Status = token, &lease, model.AnalysisProcessing
			response = &filev1.AcquireAnalysisCacheResponse{Decision: filev1.AnalysisCacheDecision_CACHE_COMPUTE, Entry: cacheEntry(result), LeaseToken: token}
			return nil
		})
		if err == nil {
			return response, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			break
		}
	}
	return nil, status.Errorf(codes.Internal, "获取分析缓存失败: %v", err)
}

func (s *Server) CompleteAnalysisCache(ctx context.Context, req *filev1.CompleteAnalysisCacheRequest) (*filev1.AnalysisCacheEntry, error) {
	if req.Id == 0 || req.LeaseToken == "" || !json.Valid([]byte(req.ResultJson)) {
		return nil, status.Error(codes.InvalidArgument, "分析结果缓存参数无效")
	}
	now := time.Now().UTC()
	updates := map[string]any{"result_json": req.ResultJson, "provider": strings.TrimSpace(req.Provider), "status": model.AnalysisComplete, "lease_token": "", "lease_expires_at": nil, "error_message": "", "last_accessed_at": now, "generated_at": now}
	if req.TtlSeconds > 0 {
		expires := now.Add(time.Duration(req.TtlSeconds) * time.Second)
		updates["expires_at"] = expires
	} else {
		updates["expires_at"] = nil
	}
	changed := s.db.WithContext(ctx).Model(&model.AnalysisResult{}).Where("id=? AND status=? AND lease_token=?", req.Id, model.AnalysisProcessing, req.LeaseToken).Updates(updates)
	if changed.Error != nil {
		return nil, status.Error(codes.Internal, "保存分析缓存失败")
	}
	if changed.RowsAffected != 1 {
		return nil, status.Error(codes.FailedPrecondition, "分析缓存租约已失效")
	}
	var result model.AnalysisResult
	if err := s.db.WithContext(ctx).First(&result, req.Id).Error; err != nil {
		return nil, status.Error(codes.Internal, "读取分析缓存失败")
	}
	return cacheEntry(result), nil
}

func (s *Server) FailAnalysisCache(ctx context.Context, req *filev1.FailAnalysisCacheRequest) (*filev1.Empty, error) {
	if req.Id == 0 || req.LeaseToken == "" {
		return nil, status.Error(codes.InvalidArgument, "分析缓存租约无效")
	}
	errorMessage := strings.TrimSpace(req.ErrorMessage)
	if len(errorMessage) > 1024 {
		errorMessage = errorMessage[:1024]
	}
	changed := s.db.WithContext(ctx).Model(&model.AnalysisResult{}).Where("id=? AND status=? AND lease_token=?", req.Id, model.AnalysisProcessing, req.LeaseToken).Updates(map[string]any{"status": model.AnalysisFailed, "lease_token": "", "lease_expires_at": nil, "error_message": errorMessage})
	if changed.Error != nil {
		return nil, status.Error(codes.Internal, "标记分析失败时发生错误")
	}
	return &filev1.Empty{}, nil
}

func cacheEntry(value model.AnalysisResult) *filev1.AnalysisCacheEntry {
	entry := &filev1.AnalysisCacheEntry{Id: value.ID, FileId: value.FileID, CacheKey: value.CacheKey, Provider: value.Provider}
	if value.GeneratedAt != nil {
		entry.GeneratedAt = value.GeneratedAt.Unix()
	}
	if value.ResultJSON != nil {
		entry.ResultJson = *value.ResultJSON
	}
	if value.ExpiresAt != nil {
		entry.ExpiresAt = value.ExpiresAt.Unix()
	}
	return entry
}

func (s *Server) HandleUploadComplete(ctx context.Context, event UploadCompleteEvent) (bool, error) {
	applied := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session model.UploadSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, event.UploadID).Error; err != nil {
			return err
		}
		if session.Status == model.UploadCancelled || session.Status == model.UploadFailed {
			return nil
		}
		var object model.FileObject
		err := tx.Where("file_hash=?", session.FileHash).First(&object).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			object = model.FileObject{FileHash: session.FileHash, FileName: session.Filename, FileSize: session.FileSize, Bucket: session.Bucket, StoragePath: session.StoragePath, Status: model.FileComplete}
			metadata := utils.ParseScientificFilename(session.Filename)
			if metadata.Classified {
				object.RegionCode, object.BlockIndex, object.DataYear = &metadata.RegionCode, &metadata.BlockIndex, &metadata.DataYear
			}
			if err := tx.Create(&object).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		userFile := model.UserFile{UserID: session.UserID, FileID: object.ID, Filename: session.Filename, Folder: session.Folder}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&userFile).Error; err != nil {
			return err
		}
		if err := tx.Model(&session).Update("status", model.UploadComplete).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	return applied, err
}

type UploadCompleteEvent struct {
	EventID     string `json:"event_id"`
	UploadID    uint64 `json:"upload_id"`
	FileHash    string `json:"file_hash"`
	StoragePath string `json:"storage_path"`
}

func (s *Server) AfterUploadComplete(ctx context.Context, event UploadCompleteEvent) {
	value, _ := json.Marshal(map[string]any{"upload_id": event.UploadID, "storage_path": event.StoragePath})
	_ = s.redis.HSet(ctx, "all_file", event.FileHash, value).Err()
	_ = s.redis.Del(ctx, "upload_session:"+event.FileHash, fmt.Sprintf("upload_heartbeat:%d", event.UploadID)).Err()
	_ = s.redis.Del(ctx, "upload_waiters:"+event.FileHash).Err()
}

func (s *Server) getSession(ctx context.Context, userID, uploadID uint64) (model.UploadSession, error) {
	var value model.UploadSession
	err := s.db.WithContext(ctx).First(&value, uploadID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, status.Error(codes.NotFound, "上传会话不存在")
	}
	if err != nil {
		return value, status.Error(codes.Internal, "查询上传会话失败")
	}
	if value.UserID != userID {
		return value, status.Error(codes.PermissionDenied, "无权访问上传会话")
	}
	return value, nil
}
func (s *Server) setStatus(ctx context.Context, req *filev1.UploadActionRequest, value int) (*filev1.UploadInfo, error) {
	session, err := s.getSession(ctx, req.UserId, req.UploadId)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&session).Update("status", value).Error; err != nil {
		return nil, status.Error(codes.Internal, "更新上传状态失败")
	}
	session.Status = int8(value)
	return toUploadInfo(session, s.cachedParts(ctx, session.ID)), nil
}
func (s *Server) touchHeartbeat(ctx context.Context, id uint64) error {
	return s.redis.Set(ctx, fmt.Sprintf("upload_heartbeat:%d", id), time.Now().Unix(), 30*time.Second).Err()
}
func (s *Server) cachedParts(ctx context.Context, id uint64) []*filev1.UploadedPart {
	values, _ := s.redis.HGetAll(ctx, fmt.Sprintf("upload_parts:%d", id)).Result()
	result := make([]*filev1.UploadedPart, 0, len(values))
	for _, value := range values {
		var part filev1.UploadedPart
		if json.Unmarshal([]byte(value), &part) == nil {
			result = append(result, &part)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PartNumber < result[j].PartNumber })
	return result
}
func (s *Server) loadParts(ctx context.Context, session model.UploadSession) []*filev1.UploadedPart {
	parts := s.cachedParts(ctx, session.ID)
	if len(parts) > 0 || session.UploadMode != model.ModeMultipart {
		return parts
	}
	remote, err := s.storage.ListParts(ctx, &storagev1.ListPartsRequest{UserId: session.UserID, UploadId: session.ID})
	if err != nil {
		return parts
	}
	for _, item := range remote.Parts {
		part := &filev1.UploadedPart{PartNumber: item.PartNumber, Etag: item.Etag, Size: item.Size, Status: "SUCCESS"}
		data, _ := json.Marshal(part)
		_ = s.redis.HSet(ctx, fmt.Sprintf("upload_parts:%d", session.ID), item.PartNumber, data).Err()
		parts = append(parts, part)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	return parts
}
func (s *Server) findUserFile(ctx context.Context, userID, id uint64) (model.UserFile, model.FileObject, error) {
	var uf model.UserFile
	if userID == 0 {
		return uf, model.FileObject{}, status.Error(codes.Unauthenticated, "用户身份无效")
	}
	if err := s.db.WithContext(ctx).Where("id=?", id).First(&uf).Error; err != nil {
		return uf, model.FileObject{}, status.Error(codes.NotFound, "文件不存在")
	}
	var fo model.FileObject
	if err := s.db.WithContext(ctx).Where("id=? AND status=?", uf.FileID, model.FileComplete).First(&fo).Error; err != nil {
		return uf, fo, status.Error(codes.NotFound, "对象不可用")
	}
	return uf, fo, nil
}

func toUploadInfo(value model.UploadSession, parts []*filev1.UploadedPart) *filev1.UploadInfo {
	statusValue := filev1.UploadStatus_INIT
	switch value.Status {
	case model.UploadUploading:
		statusValue = filev1.UploadStatus_UPLOADING
	case model.UploadComplete:
		statusValue = filev1.UploadStatus_COMPLETE
	case model.UploadFailed:
		statusValue = filev1.UploadStatus_FAILED
	case model.UploadPaused:
		statusValue = filev1.UploadStatus_PAUSED
	case model.UploadCancelled:
		statusValue = filev1.UploadStatus_CANCELLED
	case model.UploadFinalizing:
		statusValue = filev1.UploadStatus_UPLOADING
	}
	return &filev1.UploadInfo{UploadId: value.ID, UserId: value.UserID, FileHash: value.FileHash, Filename: value.Filename, Folder: value.Folder, FileSize: value.FileSize, ChunkSize: int64(value.ChunkSize), ChunkCount: int32(value.ChunkCount), Mode: filev1.UploadMode(value.UploadMode), Status: statusValue, Parts: parts}
}
func toFileInfo(uf model.UserFile, fo model.FileObject) *filev1.FileInfo {
	info := &filev1.FileInfo{Id: uf.ID, FileId: fo.ID, Filename: uf.Filename, Folder: uf.Folder, FileHash: fo.FileHash, FileSize: fo.FileSize, Status: "COMPLETE", CreatedAt: uf.CreatedAt.Unix()}
	applyFileMetadata(info, fo.RegionCode, fo.BlockIndex, fo.DataYear)
	return info
}

func applyFileMetadata(info *filev1.FileInfo, regionCode *string, blockIndex, dataYear *uint32) {
	if regionCode == nil || blockIndex == nil || dataYear == nil {
		return
	}
	block := *blockIndex
	info.RegionCode, info.BlockIndex, info.DataYear, info.Classified = *regionCode, &block, *dataYear, true
}

func metadataUpdates(value utils.ScientificMetadata) map[string]any {
	return map[string]any{"region_code": value.RegionCode, "block_index": value.BlockIndex, "data_year": value.DataYear}
}

package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	filev1 "github.com/clivegformer/platform/contracts/gen/file/v1"
	storagev1 "github.com/clivegformer/platform/contracts/gen/storage/v1"
	"github.com/clivegformer/platform/storage_srv/model"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EventSender interface{ Send([]byte) error }

type Server struct {
	storagev1.UnimplementedStorageServiceServer
	client *minio.Client
	core   *minio.Core
	files  filev1.FileServiceClient
	events EventSender
}

func New(client *minio.Client, core *minio.Core, files filev1.FileServiceClient, events EventSender) *Server {
	return &Server{client: client, core: core, files: files, events: events}
}

func (s *Server) BeginUpload(ctx context.Context, req *storagev1.BeginUploadRequest) (*storagev1.BeginUploadResponse, error) {
	exists, err := s.client.BucketExists(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "检查Ceph Bucket失败: %v", err)
	}
	if !exists {
		return nil, status.Error(codes.FailedPrecondition, "Ceph Bucket不存在")
	}
	if req.Mode == int32(filev1.UploadMode_MULTIPART) {
		uploadID, err := s.core.NewMultipartUpload(ctx, req.Bucket, req.ObjectKey, minio.PutObjectOptions{ContentType: req.ContentType})
		if err != nil {
			return nil, status.Errorf(codes.Unavailable, "创建Multipart失败: %v", err)
		}
		return &storagev1.BeginUploadResponse{CephUploadId: uploadID}, nil
	}
	return &storagev1.BeginUploadResponse{}, nil
}

func (s *Server) UploadPart(stream storagev1.StorageService_UploadPartServer) error {
	first, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "缺少分片元数据")
	}
	if first.Meta == nil || first.Meta.Size <= 0 {
		return status.Error(codes.InvalidArgument, "分片元数据不正确")
	}
	meta := first.Meta
	auth, err := s.files.AuthorizePart(stream.Context(), &filev1.AuthorizePartRequest{UserId: meta.UserId, UploadId: meta.UploadId, PartNumber: meta.PartNumber, Size: meta.Size})
	if err != nil {
		return err
	}

	reader, writer := io.Pipe()
	hasher := sha256.New()
	type result struct {
		etag string
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		if auth.Mode == filev1.UploadMode_SINGLE {
			info, uploadErr := s.client.PutObject(stream.Context(), auth.Bucket, auth.ObjectKey, reader, meta.Size, minio.PutObjectOptions{ContentType: "application/octet-stream"})
			resultCh <- result{etag: info.ETag, err: uploadErr}
			return
		}
		part, uploadErr := s.core.PutObjectPart(stream.Context(), auth.Bucket, auth.ObjectKey, auth.CephUploadId, int(meta.PartNumber), reader, meta.Size, minio.PutObjectPartOptions{})
		resultCh <- result{etag: part.ETag, err: uploadErr}
	}()

	written := int64(0)
	write := func(data []byte) error {
		if len(data) == 0 {
			return nil
		}
		if written+int64(len(data)) > meta.Size {
			return status.Error(codes.InvalidArgument, "分片超过声明大小")
		}
		n, err := io.MultiWriter(writer, hasher).Write(data)
		written += int64(n)
		return err
	}
	if err := write(first.Data); err != nil {
		_ = writer.CloseWithError(err)
		return err
	}
	for {
		frame, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			_ = writer.CloseWithError(recvErr)
			return recvErr
		}
		if err := write(frame.Data); err != nil {
			_ = writer.CloseWithError(err)
			return err
		}
	}
	if written != meta.Size {
		err := fmt.Errorf("分片大小不一致: %d/%d", written, meta.Size)
		_ = writer.CloseWithError(err)
		return status.Error(codes.InvalidArgument, err.Error())
	}
	_ = writer.Close()
	uploadResult := <-resultCh
	if uploadResult.err != nil {
		return status.Errorf(codes.Unavailable, "写入Ceph失败: %v", uploadResult.err)
	}
	if meta.ChunkSha256 != "" && !strings.EqualFold(meta.ChunkSha256, hex.EncodeToString(hasher.Sum(nil))) {
		return status.Error(codes.DataLoss, "分片SHA-256不匹配")
	}
	if _, err := s.files.RecordPart(stream.Context(), &filev1.RecordPartRequest{UploadId: meta.UploadId, PartNumber: meta.PartNumber, Etag: uploadResult.etag, Size: written}); err != nil {
		return err
	}
	return stream.SendAndClose(&storagev1.UploadPartResponse{PartNumber: meta.PartNumber, Etag: uploadResult.etag, Size: written})
}

func (s *Server) ListParts(ctx context.Context, req *storagev1.ListPartsRequest) (*storagev1.ListPartsResponse, error) {
	auth, err := s.files.ResolveUpload(ctx, &filev1.GetUploadRequest{UserId: req.UserId, UploadId: req.UploadId})
	if err != nil {
		return nil, err
	}
	if auth.Mode == filev1.UploadMode_SINGLE {
		return &storagev1.ListPartsResponse{}, nil
	}
	result, err := s.core.ListObjectParts(ctx, auth.Bucket, auth.ObjectKey, auth.CephUploadId, 0, 10000)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "读取Ceph分片失败: %v", err)
	}
	parts := make([]*storagev1.Part, 0, len(result.ObjectParts))
	for _, item := range result.ObjectParts {
		parts = append(parts, &storagev1.Part{PartNumber: int32(item.PartNumber), Etag: item.ETag, Size: item.Size})
	}
	return &storagev1.ListPartsResponse{Parts: parts}, nil
}

func (s *Server) CompleteUpload(ctx context.Context, req *storagev1.CompleteUploadRequest) (*storagev1.CompleteUploadResponse, error) {
	auth, err := s.files.ResolveUpload(ctx, &filev1.GetUploadRequest{UserId: req.UserId, UploadId: req.UploadId})
	if err != nil {
		return nil, err
	}
	etag := ""
	if auth.Mode == filev1.UploadMode_MULTIPART {
		if existing, statErr := s.client.StatObject(ctx, auth.Bucket, auth.ObjectKey, minio.StatObjectOptions{}); statErr == nil {
			etag = existing.ETag
		} else {
			sort.Slice(req.Parts, func(i, j int) bool { return req.Parts[i].PartNumber < req.Parts[j].PartNumber })
			parts := make([]minio.CompletePart, 0, len(req.Parts))
			for _, p := range req.Parts {
				parts = append(parts, minio.CompletePart{PartNumber: int(p.PartNumber), ETag: p.Etag})
			}
			info, err := s.core.CompleteMultipartUpload(ctx, auth.Bucket, auth.ObjectKey, auth.CephUploadId, parts, minio.PutObjectOptions{})
			if err != nil {
				return nil, status.Errorf(codes.Unavailable, "完成Ceph Multipart失败: %v", err)
			}
			etag = info.ETag
		}
	}
	objectInfo, err := s.client.StatObject(ctx, auth.Bucket, auth.ObjectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "完成后校验Ceph对象失败: %v", err)
	}
	if objectInfo.Size != auth.ExpectedSize {
		return nil, status.Errorf(codes.DataLoss, "Ceph对象大小不一致: %d/%d", objectInfo.Size, auth.ExpectedSize)
	}
	etag = objectInfo.ETag
	event := model.UploadCompleteEvent{EventID: uuid.NewString(), UploadID: req.UploadId, FileHash: auth.FileHash, StoragePath: auth.ObjectKey}
	body, _ := json.Marshal(event)
	if err := s.events.Send(body); err != nil {
		return nil, status.Errorf(codes.Unavailable, "发送完成消息失败: %v", err)
	}
	return &storagev1.CompleteUploadResponse{Bucket: auth.Bucket, ObjectKey: auth.ObjectKey, Etag: etag}, nil
}

func (s *Server) AbortUpload(ctx context.Context, req *storagev1.AbortUploadRequest) (*storagev1.Empty, error) {
	auth, err := s.files.ResolveUpload(ctx, &filev1.GetUploadRequest{UserId: req.UserId, UploadId: req.UploadId})
	if err != nil {
		return nil, err
	}
	if auth.Mode == filev1.UploadMode_MULTIPART && auth.CephUploadId != "" {
		err = s.core.AbortMultipartUpload(ctx, auth.Bucket, auth.ObjectKey, auth.CephUploadId)
	} else {
		err = s.client.RemoveObject(ctx, auth.Bucket, auth.ObjectKey, minio.RemoveObjectOptions{})
	}
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "终止Ceph上传失败: %v", err)
	}
	return &storagev1.Empty{}, nil
}

func (s *Server) Download(req *storagev1.DownloadRequest, stream storagev1.StorageService_DownloadServer) error {
	return s.streamObject(stream.Context(), req.UserId, req.UserFileId, req.RangeStart, req.RangeEnd, func(chunk *storagev1.DownloadChunk) error { return stream.Send(chunk) })
}
func (s *Server) ReadRange(req *storagev1.ReadRangeRequest, stream storagev1.StorageService_ReadRangeServer) error {
	end := req.Offset + req.Length - 1
	return s.streamObject(stream.Context(), req.UserId, req.UserFileId, req.Offset, end, func(chunk *storagev1.DownloadChunk) error { return stream.Send(chunk) })
}

func (s *Server) streamObject(ctx context.Context, userID, fileID uint64, start, end int64, send func(*storagev1.DownloadChunk) error) error {
	resolved, err := s.files.ResolveDownload(ctx, &filev1.ResolveDownloadRequest{UserId: userID, UserFileId: fileID})
	if err != nil {
		return err
	}
	opts := minio.GetObjectOptions{}
	if start >= 0 && end >= start {
		if err := opts.SetRange(start, end); err != nil {
			return status.Error(codes.InvalidArgument, "Range不正确")
		}
	}
	object, err := s.client.GetObject(ctx, resolved.Bucket, resolved.ObjectKey, opts)
	if err != nil {
		return status.Errorf(codes.Unavailable, "读取Ceph对象失败: %v", err)
	}
	defer object.Close()
	buffer := make([]byte, 64<<10)
	first := true
	for {
		n, readErr := object.Read(buffer)
		if n > 0 {
			chunk := &storagev1.DownloadChunk{Data: append([]byte(nil), buffer[:n]...)}
			if first {
				chunk.TotalSize = resolved.Size
				chunk.Filename = resolved.Filename
				chunk.ContentType = "application/octet-stream"
				first = false
			}
			if err := send(chunk); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return status.Errorf(codes.Unavailable, "Ceph读取中断: %v", readErr)
		}
	}
}

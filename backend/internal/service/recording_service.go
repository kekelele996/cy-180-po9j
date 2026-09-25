package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/dto"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/repository"
	"github.com/oralhistory/oralhistory/internal/util"
)

// RecordingService 录音片段业务接口。
type RecordingService interface {
	Create(actor *model.User, req *dto.CreateRecordingRequest) (*model.Recording, error)
	Get(id uint) (*model.Recording, error)
	// List 同时服务「按项目」与「按问题」两个接口，reviewStatus 非空时按摘要审核状态过滤。
	List(projectID, questionID uint, reviewStatus string) ([]model.Recording, error)
	Update(actor *model.User, id uint, req *dto.UpdateRecordingRequest) (*model.Recording, error)
	UpdateSummary(actor *model.User, id uint, summary string) (*model.Recording, error)
	AttachAudio(actor *model.User, id uint, audioKey string, duration int) (*model.Recording, error)
	// SubmitForReview 采访员把有音频的片段提交给档案员/管理员审核。
	SubmitForReview(actor *model.User, id uint) (*model.Recording, error)
	// ApproveReview 审核通过，可同时修改摘要。
	ApproveReview(actor *model.User, id uint, summary string) (*model.Recording, error)
	// RejectReview 填写意见后退回采访员修改。
	RejectReview(actor *model.User, id uint, comment string) (*model.Recording, error)
	Delete(actor *model.User, id uint) error
	CountByProject(projectID uint) (int64, error)
}

type recordingService struct {
	recordingRepo repository.RecordingRepository
	projectRepo   repository.ProjectRepository
	questionRepo  repository.QuestionRepository
	logger        *slog.Logger
}

// NewRecordingService 构造录音服务。
func NewRecordingService(recordingRepo repository.RecordingRepository, projectRepo repository.ProjectRepository, questionRepo repository.QuestionRepository, logger *slog.Logger) RecordingService {
	return &recordingService{recordingRepo: recordingRepo, projectRepo: projectRepo, questionRepo: questionRepo, logger: logger}
}

func (s *recordingService) Create(actor *model.User, req *dto.CreateRecordingRequest) (*model.Recording, error) {
	if _, err := s.projectRepo.FindByID(req.ProjectID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("项目 %d 不存在", req.ProjectID), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询项目 %d 失败", req.ProjectID), err)
	}
	if _, err := s.questionRepo.FindByID(req.QuestionID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("问题 %d 不存在", req.QuestionID), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询问题 %d 失败", req.QuestionID), err)
	}
	recording := &model.Recording{
		ProjectID:       req.ProjectID,
		QuestionID:      req.QuestionID,
		DurationSeconds: req.DurationSeconds,
		Summary:         req.Summary,
		Status:          constants.RecordingStatusRecording,
		ReviewStatus:    constants.ReviewStatusDraft,
		CreatedBy:       actor.ID,
	}
	if err := s.recordingRepo.Create(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("创建问题 %d 的录音失败", req.QuestionID), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingUpload, actor.Username, recording.ProjectID, recording.QuestionID, recording.DurationSeconds, recording.Status))
	return recording, nil
}

func (s *recordingService) Get(id uint) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	return recording, nil
}

func (s *recordingService) List(projectID, questionID uint, reviewStatus string) ([]model.Recording, error) {
	if reviewStatus != "" && !constants.ValidReviewStatus(reviewStatus) {
		return nil, util.NewAppError(constants.CodeValidation, fmt.Sprintf("摘要审核状态 %s 不合法", reviewStatus), nil)
	}
	var (
		recordings []model.Recording
		err        error
	)
	if projectID > 0 {
		if reviewStatus != "" {
			recordings, err = s.recordingRepo.ListByProjectReviewStatus(projectID, reviewStatus)
		} else {
			recordings, err = s.recordingRepo.ListByProject(projectID)
		}
	} else {
		recordings, err = s.recordingRepo.ListByQuestion(questionID)
		if err == nil && reviewStatus != "" {
			filtered := recordings[:0]
			for _, r := range recordings {
				if r.ReviewStatus == reviewStatus {
					filtered = append(filtered, r)
				}
			}
			recordings = filtered
		}
	}
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternal, "录音列表查询失败", err)
	}
	return recordings, nil
}

func (s *recordingService) Update(actor *model.User, id uint, req *dto.UpdateRecordingRequest) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	if req.DurationSeconds > 0 {
		recording.DurationSeconds = req.DurationSeconds
	}
	if req.Summary != "" {
		recording.Summary = req.Summary
	}
	if req.Status != "" {
		if !constants.ValidRecordingStatus(req.Status) {
			return nil, util.NewAppError(constants.CodeValidation, fmt.Sprintf("录音状态 %s 不合法", req.Status), nil)
		}
		if !constants.CanTransitionRecording(recording.Status, req.Status) {
			return nil, util.NewAppError(constants.CodeRecordingStatus,
				fmt.Sprintf("录音 %d 状态不允许从 %s 流转到 %s", id, recording.Status, req.Status), nil)
		}
		recording.Status = req.Status
	}
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("更新录音 %d 失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingStatus, actor.Username, recording.ID, recording.Status, recording.Status))
	return recording, nil
}

func (s *recordingService) UpdateSummary(actor *model.User, id uint, summary string) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	// 已提交待审核的摘要由档案员处理，采访员需等待审核结果或退回后再改。
	if recording.ReviewStatus == constants.ReviewStatusPending {
		return nil, util.NewAppError(constants.CodeReviewStatus,
			fmt.Sprintf("录音 %d 的摘要正在审核中，暂不能修改", id), nil)
	}
	recording.Summary = summary
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("更新录音 %d 摘要失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingSummary, actor.Username, recording.ID, summary))
	return recording, nil
}

func (s *recordingService) AttachAudio(actor *model.User, id uint, audioKey string, duration int) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByIDForUpdate(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	recording.AudioKey = audioKey
	if duration > 0 {
		recording.DurationSeconds = duration
	}
	if recording.Status == constants.RecordingStatusRecording || recording.Status == constants.RecordingStatusProcessing {
		recording.Status = constants.RecordingStatusReady
	}
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("录音 %d 音频关联失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingUpload, actor.Username, recording.ProjectID, recording.QuestionID, recording.DurationSeconds, recording.Status))
	return recording, nil
}

func (s *recordingService) SubmitForReview(actor *model.User, id uint) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByIDForUpdate(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	if recording.AudioKey == "" {
		return nil, util.NewAppError(constants.CodeValidation, fmt.Sprintf("录音 %d 尚无音频文件，不能提交审核", id), nil)
	}
	if strings.TrimSpace(recording.Summary) == "" {
		return nil, util.NewAppError(constants.CodeValidation, fmt.Sprintf("录音 %d 还没有摘要，请先填写一句话摘要再提交", id), nil)
	}
	if !constants.CanSubmitReview(recording.ReviewStatus) {
		return nil, util.NewAppError(constants.CodeReviewStatus,
			fmt.Sprintf("录音 %d 摘要当前为 %s 状态，不能提交审核", id, recording.ReviewStatus), nil)
	}
	from := recording.ReviewStatus
	recording.ReviewStatus = constants.ReviewStatusPending
	recording.ReviewComment = ""
	recording.ReviewedBy = 0
	recording.ReviewedAt = nil
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("录音 %d 提交审核失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogReviewSubmit, actor.Username, id, from, recording.ReviewStatus))
	return recording, nil
}

func (s *recordingService) ApproveReview(actor *model.User, id uint, summary string) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByIDForUpdate(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	if !constants.CanReview(recording.ReviewStatus) {
		return nil, util.NewAppError(constants.CodeReviewStatus,
			fmt.Sprintf("录音 %d 摘要当前为 %s 状态，不能审核通过", id, recording.ReviewStatus), nil)
	}
	if summary != "" {
		recording.Summary = summary
	}
	now := time.Now()
	recording.ReviewStatus = constants.ReviewStatusApproved
	recording.ReviewComment = ""
	recording.ReviewedBy = actor.ID
	recording.ReviewedAt = &now
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("录音 %d 审核通过失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogReviewApprove, actor.Username, id, recording.Summary))
	return recording, nil
}

func (s *recordingService) RejectReview(actor *model.User, id uint, comment string) (*model.Recording, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return nil, util.NewAppError(constants.CodeValidation, "退回时必须填写审核意见", nil)
	}
	recording, err := s.recordingRepo.FindByIDForUpdate(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	if !constants.CanReview(recording.ReviewStatus) {
		return nil, util.NewAppError(constants.CodeReviewStatus,
			fmt.Sprintf("录音 %d 摘要当前为 %s 状态，不能退回", id, recording.ReviewStatus), nil)
	}
	now := time.Now()
	recording.ReviewStatus = constants.ReviewStatusRejected
	recording.ReviewComment = comment
	recording.ReviewedBy = actor.ID
	recording.ReviewedAt = &now
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("录音 %d 退回失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogReviewReject, actor.Username, id, comment))
	return recording, nil
}

func (s *recordingService) Delete(actor *model.User, id uint) error {
	if _, err := s.recordingRepo.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	if err := s.recordingRepo.Delete(id); err != nil {
		return util.NewAppError(constants.CodeInternal, fmt.Sprintf("删除录音 %d 失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingDelete, actor.Username, id))
	return nil
}

func (s *recordingService) CountByProject(projectID uint) (int64, error) {
	return s.recordingRepo.CountByProject(projectID)
}

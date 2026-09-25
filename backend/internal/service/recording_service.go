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
	// List 同时服务「按项目」与「按问题」两个接口，复用同一 service 方法；status 非空时按状态筛选。
	List(projectID, questionID uint, status string) ([]model.Recording, error)
	Update(actor *model.User, id uint, req *dto.UpdateRecordingRequest) (*model.Recording, error)
	UpdateSummary(actor *model.User, id uint, summary string) (*model.Recording, error)
	AttachAudio(actor *model.User, id uint, audioKey string, duration int) (*model.Recording, error)
	// SubmitForReview 采访员把有音频的片段提交给档案员/管理员审核。
	SubmitForReview(actor *model.User, id uint) (*model.Recording, error)
	// Review 档案员/管理员审核摘要：可修改摘要，通过或填写意见后退回。
	Review(actor *model.User, id uint, req *dto.ReviewRecordingRequest) (*model.Recording, error)
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

func (s *recordingService) List(projectID, questionID uint, status string) ([]model.Recording, error) {
	if status != "" && !constants.ValidRecordingStatus(status) {
		return nil, util.NewAppError(constants.CodeValidation, fmt.Sprintf("录音状态 %s 不合法", status), nil)
	}
	var (
		recordings []model.Recording
		err        error
	)
	if projectID > 0 {
		recordings, err = s.recordingRepo.ListByProject(projectID, status)
	} else {
		recordings, err = s.recordingRepo.ListByQuestion(questionID, status)
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
		// 审核相关状态只能通过提交/审核接口流转，确保音频、摘要、审核意见等校验生效。
		if req.Status == constants.RecordingStatusPending ||
			req.Status == constants.RecordingStatusApproved ||
			req.Status == constants.RecordingStatusRejected {
			return nil, util.NewAppError(constants.CodeRecordingStatus,
				fmt.Sprintf("录音 %d 状态 %s 必须通过提交审核/审核接口流转", id, req.Status), nil)
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
	switch recording.Status {
	case constants.RecordingStatusPending:
		return nil, util.NewAppError(constants.CodeRecordingStatus,
			fmt.Sprintf("录音 %d 摘要正在审核中，暂不能修改", id), nil)
	case constants.RecordingStatusApproved:
		return nil, util.NewAppError(constants.CodeRecordingStatus,
			fmt.Sprintf("录音 %d 摘要已审核通过，不能再修改", id), nil)
	}
	recording.Summary = summary
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("更新录音 %d 摘要失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingSummary, actor.Username, recording.ID, summary))
	return recording, nil
}

// SubmitForReview 采访员提交有音频的片段进入摘要审核。
func (s *recordingService) SubmitForReview(actor *model.User, id uint) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByIDForUpdate(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	if recording.AudioKey == "" {
		return nil, util.NewAppError(constants.CodeValidation,
			fmt.Sprintf("录音 %d 尚未上传音频，不能提交审核", id), nil)
	}
	if strings.TrimSpace(recording.Summary) == "" {
		return nil, util.NewAppError(constants.CodeValidation,
			fmt.Sprintf("录音 %d 尚未填写摘要，不能提交审核", id), nil)
	}
	if !constants.CanTransitionRecording(recording.Status, constants.RecordingStatusPending) {
		return nil, util.NewAppError(constants.CodeRecordingStatus,
			fmt.Sprintf("录音 %d 当前状态 %s 不允许提交审核", id, recording.Status), nil)
	}
	recording.Status = constants.RecordingStatusPending
	// 重新提交时清空上一轮审核痕迹。
	recording.ReviewComment = ""
	recording.ReviewedBy = 0
	recording.ReviewedAt = nil
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("录音 %d 提交审核失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingReviewSubmit, actor.Username, recording.ID, recording.Status))
	return recording, nil
}

// Review 档案员/管理员审核摘要：通过，或修改摘要/填写意见后退回。
func (s *recordingService) Review(actor *model.User, id uint, req *dto.ReviewRecordingRequest) (*model.Recording, error) {
	recording, err := s.recordingRepo.FindByIDForUpdate(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("录音 %d 不存在", id), err)
		}
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("查询录音 %d 失败", id), err)
	}
	if recording.Status != constants.RecordingStatusPending {
		return nil, util.NewAppError(constants.CodeRecordingStatus,
			fmt.Sprintf("录音 %d 当前状态 %s，不在待审核状态", id, recording.Status), nil)
	}
	if !req.Approved && strings.TrimSpace(req.Comment) == "" {
		return nil, util.NewAppError(constants.CodeValidation, "退回审核时必须填写审核意见", nil)
	}
	if req.Summary != "" {
		recording.Summary = req.Summary
	}
	now := time.Now()
	recording.ReviewedBy = actor.ID
	recording.ReviewedAt = &now
	decision := constants.RecordingStatusApproved
	if req.Approved {
		recording.Status = constants.RecordingStatusApproved
		recording.ReviewComment = strings.TrimSpace(req.Comment)
	} else {
		recording.Status = constants.RecordingStatusRejected
		recording.ReviewComment = strings.TrimSpace(req.Comment)
		decision = constants.RecordingStatusRejected
	}
	if err := s.recordingRepo.Update(recording); err != nil {
		return nil, util.NewAppError(constants.CodeInternal, fmt.Sprintf("录音 %d 摘要审核失败", id), err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogRecordingReview, actor.Username, recording.ID, decision, recording.ReviewComment))
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

package service

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/dto"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/util"
)

type fakeRecordingRepoForReview struct {
	recording *model.Recording
}

func (f *fakeRecordingRepoForReview) Create(recording *model.Recording) error { return nil }
func (f *fakeRecordingRepoForReview) FindByID(id uint) (*model.Recording, error) {
	return f.recording, nil
}
func (f *fakeRecordingRepoForReview) ListByProject(projectID uint, status string) ([]model.Recording, error) {
	return nil, nil
}
func (f *fakeRecordingRepoForReview) ListByQuestion(questionID uint, status string) ([]model.Recording, error) {
	return nil, nil
}
func (f *fakeRecordingRepoForReview) FindByIDForUpdate(id uint) (*model.Recording, error) {
	return f.recording, nil
}
func (f *fakeRecordingRepoForReview) Update(recording *model.Recording) error {
	f.recording = recording
	return nil
}
func (f *fakeRecordingRepoForReview) UpdateStatus(recording *model.Recording) error {
	f.recording = recording
	return nil
}
func (f *fakeRecordingRepoForReview) Delete(id uint) error                         { return nil }
func (f *fakeRecordingRepoForReview) CountByProject(projectID uint) (int64, error) { return 0, nil }
func (f *fakeRecordingRepoForReview) CountPendingReviewByProject(projectID uint) (int64, error) {
	return 0, nil
}

func newReviewService(recording *model.Recording) RecordingService {
	return NewRecordingService(
		&fakeRecordingRepoForReview{recording: recording},
		&fakeProjectRepo{projects: map[uint]*model.Project{1: {ID: 1, Status: constants.ProjectStatusInProgress}}},
		&fakeQuestionRepoForReview{},
		slog.Default(),
	)
}

type fakeQuestionRepoForReview struct{}

func (f *fakeQuestionRepoForReview) Create(question *model.Question) error { return nil }
func (f *fakeQuestionRepoForReview) FindByID(id uint) (*model.Question, error) {
	return &model.Question{ID: id}, nil
}
func (f *fakeQuestionRepoForReview) ListByProject(projectID uint) ([]model.Question, error) {
	return nil, nil
}
func (f *fakeQuestionRepoForReview) Update(question *model.Question) error { return nil }
func (f *fakeQuestionRepoForReview) Delete(id uint) error                  { return nil }
func (f *fakeQuestionRepoForReview) CountByProject(projectID uint) (int64, error) {
	return 0, nil
}

func TestRecordingSubmitForReview(t *testing.T) {
	actor := &model.User{ID: 10, Username: "interviewer", Role: constants.RoleInterviewer}

	t.Run("ready clip with audio and summary can submit", func(t *testing.T) {
		svc := newReviewService(&model.Recording{
			ID: 1, AudioKey: "recordings/1/1_a.webm", Summary: "讲到童年", Status: constants.RecordingStatusReady,
		})
		got, err := svc.SubmitForReview(actor, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Status != constants.RecordingStatusPending {
			t.Fatalf("status = %s, want pending_review", got.Status)
		}
	})

	t.Run("clip without audio rejected", func(t *testing.T) {
		svc := newReviewService(&model.Recording{ID: 1, Summary: "讲到童年", Status: constants.RecordingStatusReady})
		_, err := svc.SubmitForReview(actor, 1)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *util.AppError
		if !asAppError(err, &appErr) || appErr.Code != constants.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("clip without summary rejected", func(t *testing.T) {
		svc := newReviewService(&model.Recording{ID: 1, AudioKey: "k", Status: constants.RecordingStatusReady})
		_, err := svc.SubmitForReview(actor, 1)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *util.AppError
		if !asAppError(err, &appErr) || appErr.Code != constants.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("already pending rejected", func(t *testing.T) {
		svc := newReviewService(&model.Recording{
			ID: 1, AudioKey: "k", Summary: "s", Status: constants.RecordingStatusPending,
		})
		_, err := svc.SubmitForReview(actor, 1)
		var appErr *util.AppError
		if !asAppError(err, &appErr) || appErr.Code != constants.CodeRecordingStatus {
			t.Fatalf("expected recording status error, got %v", err)
		}
	})

	t.Run("rejected clip resubmits and clears comment", func(t *testing.T) {
		reviewer := "摘要与音频内容不符"
		svc := newReviewService(&model.Recording{
			ID: 1, AudioKey: "k", Summary: "s", Status: constants.RecordingStatusRejected,
			ReviewComment: reviewer, ReviewedBy: 99,
		})
		got, err := svc.SubmitForReview(actor, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Status != constants.RecordingStatusPending || got.ReviewComment != "" || got.ReviewedBy != 0 {
			t.Fatalf("resubmit should clear review traces, got status=%s comment=%q reviewer=%d",
				got.Status, got.ReviewComment, got.ReviewedBy)
		}
	})
}

func TestRecordingReview(t *testing.T) {
	archivist := &model.User{ID: 20, Username: "archivist", Role: constants.RoleArchivist}

	t.Run("approve with summary edit", func(t *testing.T) {
		svc := newReviewService(&model.Recording{
			ID: 1, AudioKey: "k", Summary: "旧摘要", Status: constants.RecordingStatusPending,
		})
		got, err := svc.Review(archivist, 1, &dto.ReviewRecordingRequest{Approved: true, Summary: "档案员润色后的摘要"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Status != constants.RecordingStatusApproved {
			t.Fatalf("status = %s, want approved", got.Status)
		}
		if got.Summary != "档案员润色后的摘要" || got.ReviewedBy != archivist.ID || got.ReviewedAt == nil {
			t.Fatalf("approved recording should carry edited summary and reviewer")
		}
	})

	t.Run("reject requires comment", func(t *testing.T) {
		svc := newReviewService(&model.Recording{
			ID: 1, AudioKey: "k", Summary: "s", Status: constants.RecordingStatusPending,
		})
		_, err := svc.Review(archivist, 1, &dto.ReviewRecordingRequest{Approved: false})
		var appErr *util.AppError
		if !asAppError(err, &appErr) || appErr.Code != constants.CodeValidation {
			t.Fatalf("expected validation error for missing comment, got %v", err)
		}
		if !strings.Contains(appErr.Message, "审核意见") {
			t.Fatalf("error message should mention 审核意见, got %q", appErr.Message)
		}
	})

	t.Run("reject with comment", func(t *testing.T) {
		svc := newReviewService(&model.Recording{
			ID: 1, AudioKey: "k", Summary: "s", Status: constants.RecordingStatusPending,
		})
		got, err := svc.Review(archivist, 1, &dto.ReviewRecordingRequest{Approved: false, Comment: "摘要太笼统，请补充人物细节"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Status != constants.RecordingStatusRejected || got.ReviewComment == "" || got.ReviewedAt == nil {
			t.Fatalf("rejected recording should carry comment and review time")
		}
	})

	t.Run("non-pending clip cannot be reviewed", func(t *testing.T) {
		svc := newReviewService(&model.Recording{
			ID: 1, AudioKey: "k", Summary: "s", Status: constants.RecordingStatusReady,
		})
		_, err := svc.Review(archivist, 1, &dto.ReviewRecordingRequest{Approved: true})
		var appErr *util.AppError
		if !asAppError(err, &appErr) || appErr.Code != constants.CodeRecordingStatus {
			t.Fatalf("expected recording status error, got %v", err)
		}
	})
}

func TestRecordingGenericUpdateCannotBypassReview(t *testing.T) {
	actor := &model.User{ID: 10, Username: "interviewer", Role: constants.RoleInterviewer}
	svc := newReviewService(&model.Recording{
		ID: 1, AudioKey: "k", Summary: "s", Status: constants.RecordingStatusReady,
	})
	_, err := svc.Update(actor, 1, &dto.UpdateRecordingRequest{Status: constants.RecordingStatusApproved})
	var appErr *util.AppError
	if !asAppError(err, &appErr) || appErr.Code != constants.CodeRecordingStatus {
		t.Fatalf("generic update must not jump to approved, got %v", err)
	}
}

func TestRecordingStatusTransitions(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{constants.RecordingStatusRecording, constants.RecordingStatusProcessing, true},
		{constants.RecordingStatusProcessing, constants.RecordingStatusReady, true},
		{constants.RecordingStatusReady, constants.RecordingStatusPending, true},
		{constants.RecordingStatusPending, constants.RecordingStatusApproved, true},
		{constants.RecordingStatusPending, constants.RecordingStatusRejected, true},
		{constants.RecordingStatusRejected, constants.RecordingStatusPending, true},
		{constants.RecordingStatusApproved, constants.RecordingStatusPending, false},
		{constants.RecordingStatusReady, constants.RecordingStatusApproved, false},
	}
	for _, tc := range cases {
		if got := constants.CanTransitionRecording(tc.from, tc.to); got != tc.want {
			t.Fatalf("CanTransitionRecording(%s,%s)=%v want %v", tc.from, tc.to, got, tc.want)
		}
	}
	if !constants.ValidRecordingStatus(constants.RecordingStatusPending) {
		t.Fatalf("pending_review should be a valid status")
	}
}

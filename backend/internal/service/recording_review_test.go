package service

import (
	"log/slog"
	"testing"

	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/model"
	"github.com/oralhistory/oralhistory/internal/repository"
	"github.com/oralhistory/oralhistory/internal/util"
)

type fakeRecordingRepo struct {
	recordings map[uint]*model.Recording
}

func (f *fakeRecordingRepo) Create(recording *model.Recording) error {
	f.recordings[recording.ID] = recording
	return nil
}
func (f *fakeRecordingRepo) FindByID(id uint) (*model.Recording, error) {
	if r, ok := f.recordings[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeRecordingRepo) ListByProject(projectID uint) ([]model.Recording, error) {
	return nil, nil
}
func (f *fakeRecordingRepo) ListByQuestion(questionID uint) ([]model.Recording, error) {
	return nil, nil
}
func (f *fakeRecordingRepo) ListByProjectReviewStatus(projectID uint, reviewStatus string) ([]model.Recording, error) {
	return nil, nil
}
func (f *fakeRecordingRepo) FindByIDForUpdate(id uint) (*model.Recording, error) {
	if r, ok := f.recordings[id]; ok {
		return r, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeRecordingRepo) Update(recording *model.Recording) error {
	cp := *recording
	f.recordings[recording.ID] = &cp
	return nil
}
func (f *fakeRecordingRepo) UpdateStatus(recording *model.Recording) error {
	return f.Update(recording)
}
func (f *fakeRecordingRepo) Delete(id uint) error {
	delete(f.recordings, id)
	return nil
}
func (f *fakeRecordingRepo) CountByProject(projectID uint) (int64, error) {
	return int64(len(f.recordings)), nil
}
func (f *fakeRecordingRepo) CountReviewByProject(projectID uint, statuses []string) (int64, error) {
	var total int64
	for _, r := range f.recordings {
		for _, st := range statuses {
			if r.ReviewStatus == st {
				total++
				break
			}
		}
	}
	return total, nil
}
func (f *fakeRecordingRepo) CountReviewGroupedByProject(projectID uint) (map[string]int64, error) {
	counts := map[string]int64{}
	for _, r := range f.recordings {
		counts[r.ReviewStatus]++
	}
	return counts, nil
}

func newReviewService(recordingRepo *fakeRecordingRepo) RecordingService {
	return NewRecordingService(recordingRepo, nil, nil, slog.Default())
}

func TestReviewHappyPath(t *testing.T) {
	repo := &fakeRecordingRepo{recordings: map[uint]*model.Recording{
		1: {ID: 1, ProjectID: 10, QuestionID: 100, AudioKey: "recordings/1.webm", Summary: "初始摘要", Status: constants.RecordingStatusReady, ReviewStatus: constants.ReviewStatusDraft},
	}}
	svc := newReviewService(repo)
	interviewer := &model.User{ID: 2, Role: constants.RoleInterviewer}
	archivist := &model.User{ID: 3, Role: constants.RoleArchivist}

	submitted, err := svc.SubmitForReview(interviewer, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.ReviewStatus != constants.ReviewStatusPending {
		t.Fatalf("status = %s, want pending", submitted.ReviewStatus)
	}

	approved, err := svc.ApproveReview(archivist, 1, "档案员修改后的摘要")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.ReviewStatus != constants.ReviewStatusApproved {
		t.Fatalf("status = %s, want approved", approved.ReviewStatus)
	}
	if approved.Summary != "档案员修改后的摘要" {
		t.Fatalf("summary = %s", approved.Summary)
	}
	if approved.ReviewedBy != archivist.ID || approved.ReviewedAt == nil {
		t.Fatalf("reviewer metadata not set: %+v", approved)
	}
}

func TestReviewRejectAndResubmit(t *testing.T) {
	repo := &fakeRecordingRepo{recordings: map[uint]*model.Recording{
		1: {ID: 1, AudioKey: "k", Summary: "s", ReviewStatus: constants.ReviewStatusPending},
	}}
	svc := newReviewService(repo)
	archivist := &model.User{ID: 3, Role: constants.RoleArchivist}
	interviewer := &model.User{ID: 2, Role: constants.RoleInterviewer}

	rejected, err := svc.RejectReview(archivist, 1, "摘要太简略")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.ReviewStatus != constants.ReviewStatusRejected || rejected.ReviewComment != "摘要太简略" {
		t.Fatalf("unexpected: %+v", rejected)
	}

	// 退回意见不能为空。
	if _, err := svc.RejectReview(archivist, 1, "   "); err == nil {
		t.Fatalf("expected blank comment to be rejected")
	}
	// 已退回的片段不能直接再退回/通过。
	if _, err := svc.ApproveReview(archivist, 1, ""); err == nil {
		t.Fatalf("expected approve on rejected to fail")
	}

	submitted, err := svc.SubmitForReview(interviewer, 1)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if submitted.ReviewStatus != constants.ReviewStatusPending {
		t.Fatalf("status = %s, want pending", submitted.ReviewStatus)
	}
}

func TestSubmitReviewGuard(t *testing.T) {
	cases := []struct {
		name      string
		recording *model.Recording
	}{
		{name: "no audio", recording: &model.Recording{ID: 1, Summary: "有摘要", ReviewStatus: constants.ReviewStatusDraft}},
		{name: "no summary", recording: &model.Recording{ID: 1, AudioKey: "k", ReviewStatus: constants.ReviewStatusDraft}},
		{name: "already pending", recording: &model.Recording{ID: 1, AudioKey: "k", Summary: "s", ReviewStatus: constants.ReviewStatusPending}},
		{name: "already approved", recording: &model.Recording{ID: 1, AudioKey: "k", Summary: "s", ReviewStatus: constants.ReviewStatusApproved}},
	}
	actor := &model.User{ID: 2, Role: constants.RoleInterviewer}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRecordingRepo{recordings: map[uint]*model.Recording{1: tc.recording}}
			svc := newReviewService(repo)
			_, err := svc.SubmitForReview(actor, 1)
			if err == nil {
				t.Fatalf("expected error")
			}
			var appErr *util.AppError
			if !asAppError(err, &appErr) {
				t.Fatalf("expected app error, got %v", err)
			}
		})
	}
}

func TestPendingSummaryLocked(t *testing.T) {
	repo := &fakeRecordingRepo{recordings: map[uint]*model.Recording{
		1: {ID: 1, AudioKey: "k", Summary: "s", ReviewStatus: constants.ReviewStatusPending},
	}}
	svc := newReviewService(repo)
	actor := &model.User{ID: 2, Role: constants.RoleInterviewer}
	if _, err := svc.UpdateSummary(actor, 1, "新摘要"); err == nil {
		t.Fatalf("expected summary update while pending to fail")
	}
}

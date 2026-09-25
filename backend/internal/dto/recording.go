package dto

// CreateRecordingRequest 创建录音记录请求。
type CreateRecordingRequest struct {
	ProjectID       uint   `json:"project_id" binding:"required"`
	QuestionID      uint   `json:"question_id" binding:"required"`
	DurationSeconds int    `json:"duration_seconds" binding:"omitempty,min=0"`
	Summary         string `json:"summary" binding:"omitempty,max=512"`
}

// UpdateRecordingRequest 更新录音信息请求。
type UpdateRecordingRequest struct {
	DurationSeconds int    `json:"duration_seconds" binding:"omitempty,min=0"`
	Summary         string `json:"summary" binding:"omitempty,max=512"`
	Status          string `json:"status" binding:"omitempty,oneof=recording processing ready failed pending_review rejected approved"`
}

// ReviewRecordingRequest 摘要审核请求：approved=true 通过；approved=false 退回，comment 必填。
type ReviewRecordingRequest struct {
	Approved bool   `json:"approved"`
	Summary  string `json:"summary" binding:"omitempty,max=512"`
	Comment  string `json:"comment" binding:"omitempty,max=512"`
}

// RecordingResponse 录音响应。
type RecordingResponse struct {
	ID              uint   `json:"id"`
	ProjectID       uint   `json:"project_id"`
	QuestionID      uint   `json:"question_id"`
	AudioKey        string `json:"audio_key"`
	DurationSeconds int    `json:"duration_seconds"`
	Summary         string `json:"summary"`
	Status          string `json:"status"`
	CreatedBy       uint   `json:"created_by"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

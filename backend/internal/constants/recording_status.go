// 录音片段状态机枚举。
package constants

// 录音状态定义。
const (
	RecordingStatusRecording  = "recording"      // 录制中：浏览器端正在采集
	RecordingStatusProcessing = "processing"     // 处理中：已上传等待转码
	RecordingStatusReady      = "ready"          // 就绪：可播放，摘要待提交审核
	RecordingStatusFailed     = "failed"         // 失败：上传或处理失败
	RecordingStatusPending    = "pending_review" // 待审核：采访员已提交，等待档案员/管理员审核
	RecordingStatusRejected   = "rejected"       // 已退回：审核未通过，采访员可修改后重新提交
	RecordingStatusApproved   = "approved"       // 已通过：摘要审核通过，片段进入项目时间轴
)

// ValidRecordingStatus 校验录音状态是否合法。
func ValidRecordingStatus(status string) bool {
	switch status {
	case RecordingStatusRecording, RecordingStatusProcessing, RecordingStatusReady, RecordingStatusFailed,
		RecordingStatusPending, RecordingStatusRejected, RecordingStatusApproved:
		return true
	default:
		return false
	}
}

// CanTransitionRecording 返回录音状态流转是否允许。
func CanTransitionRecording(from, to string) bool {
	switch from {
	case RecordingStatusRecording:
		return to == RecordingStatusProcessing || to == RecordingStatusFailed
	case RecordingStatusProcessing:
		return to == RecordingStatusReady || to == RecordingStatusFailed
	case RecordingStatusReady:
		return to == RecordingStatusFailed || to == RecordingStatusPending
	case RecordingStatusPending:
		return to == RecordingStatusApproved || to == RecordingStatusRejected
	case RecordingStatusRejected:
		return to == RecordingStatusPending
	case RecordingStatusApproved:
		return false
	case RecordingStatusFailed:
		return false
	default:
		return false
	}
}

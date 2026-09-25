// 录音片段摘要审核状态机枚举，与录音上传处理状态（status）相互独立。
package constants

// 摘要审核状态定义。
const (
	ReviewStatusDraft    = "draft"    // 待提交：采访员还在录音/整理摘要
	ReviewStatusPending  = "pending"  // 待审核：已提交给档案员/管理员
	ReviewStatusApproved = "approved" // 已通过：进入项目时间轴
	ReviewStatusRejected = "rejected" // 已退回：档案员填写了修改意见
)

// ValidReviewStatus 校验摘要审核状态是否合法。
func ValidReviewStatus(status string) bool {
	switch status {
	case ReviewStatusDraft, ReviewStatusPending, ReviewStatusApproved, ReviewStatusRejected:
		return true
	default:
		return false
	}
}

// CanSubmitReview 返回提交审核是否允许（仅待提交/已退回可再次提交）。
func CanSubmitReview(from string) bool {
	return from == ReviewStatusDraft || from == ReviewStatusRejected
}

// CanReview 返回档案员审核（通过/退回）是否允许（仅待审核可操作）。
func CanReview(from string) bool {
	return from == ReviewStatusPending
}

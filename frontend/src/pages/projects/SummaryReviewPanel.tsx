// 摘要审核面板：采访员提交有音频的片段；档案员/管理员修改摘要、通过或退回。
import { useState } from 'react'
import AudioPlayer from '../../components/AudioPlayer'
import EmptyState from '../../components/EmptyState'
import StatusBadge from '../../components/StatusBadge'
import {
  RECORDING_STATUS_APPROVED,
  RECORDING_STATUS_PENDING,
  RECORDING_STATUS_READY,
  RECORDING_STATUS_REJECTED,
  ROLE_ADMIN,
  ROLE_ARCHIVIST,
} from '../../constants'
import { useAuthStore } from '../../stores/authStore'
import { useQuestionStore } from '../../stores/questionStore'
import { useRecordingStore } from '../../stores/recordingStore'
import { formatDuration } from '../../utils/format'
import type { Recording } from '../../api/types'

type Notify = (message: string, type?: 'success' | 'error') => void

export default function SummaryReviewPanel({
  recordings,
  projectArchived,
  onNotify,
}: {
  recordings: Recording[]
  projectArchived: boolean
  onNotify: Notify
}) {
  const hasRole = useAuthStore((s) => s.hasRole)
  const isReviewer = hasRole(ROLE_ARCHIVIST, ROLE_ADMIN)

  if (recordings.length === 0) {
    return (
      <EmptyState
        title="暂无需审核的片段"
        description="采访员在采访工作台填写摘要并提交后，片段会进入这里等待档案员或管理员审核"
      />
    )
  }

  return (
    <div className="recording-list">
      {recordings.map((r) =>
        isReviewer ? (
          <ReviewerReviewItem key={r.id} recording={r} disabled={projectArchived} onNotify={onNotify} />
        ) : (
          <InterviewerReviewItem key={r.id} recording={r} disabled={projectArchived} onNotify={onNotify} />
        ),
      )}
    </div>
  )
}

function RecordingHead({ recording }: { recording: Recording }) {
  const question = useQuestionStore((s) => s.questions.find((q) => q.id === recording.question_id))
  return (
    <div className="timeline-head">
      <span className="timeline-q">
        {question ? `问题：${question.content}` : `问题 #${recording.question_id}`}
      </span>
      <StatusBadge status={recording.status} type="recording" />
      {recording.audio_key && <span className="timeline-duration">{formatDuration(recording.duration_seconds)}</span>}
    </div>
  )
}

function InterviewerReviewItem({
  recording,
  disabled,
  onNotify,
}: {
  recording: Recording
  disabled: boolean
  onNotify: Notify
}) {
  const { updateSummary, submitForReview } = useRecordingStore()
  const [summary, setSummary] = useState(recording.summary)
  const [saving, setSaving] = useState(false)
  const editable = !disabled && [RECORDING_STATUS_READY, RECORDING_STATUS_REJECTED].includes(recording.status)

  const saveSummary = async () => {
    const trimmed = summary.trim()
    if (!trimmed) {
      onNotify('请先填写一句话摘要', 'error')
      return
    }
    setSaving(true)
    try {
      await updateSummary(recording.id, trimmed)
      onNotify('摘要已保存')
    } catch (e) {
      onNotify(e instanceof Error ? e.message : '摘要保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  const submit = async () => {
    if (!recording.audio_key) {
      onNotify('该片段还没有音频，不能提交审核', 'error')
      return
    }
    if (!summary.trim()) {
      onNotify('请先填写一句话摘要再提交', 'error')
      return
    }
    setSaving(true)
    try {
      if (summary.trim() !== recording.summary) {
        await updateSummary(recording.id, summary.trim())
      }
      await submitForReview(recording.id)
      onNotify(recording.status === RECORDING_STATUS_REJECTED ? '已重新提交审核' : '已提交审核，等待档案员处理')
    } catch (e) {
      onNotify(e instanceof Error ? e.message : '提交审核失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="recording-row">
      <RecordingHead recording={recording} />
      {!recording.audio_key ? (
        <div className="muted">音频尚未上传，暂不能提交审核</div>
      ) : (
        <AudioPlayer recordingId={recording.id} durationSeconds={recording.duration_seconds} />
      )}

      {recording.status === RECORDING_STATUS_PENDING && (
        <div className="timeline-summary">
          <span className="summary-label">一句话摘要：</span>
          {recording.summary || <span className="muted">暂无摘要</span>}
          <div className="review-meta">已提交，正在等待档案员或管理员审核</div>
        </div>
      )}

      {recording.status === RECORDING_STATUS_REJECTED && (
        <div className="review-comment">
          审核退回意见：{recording.review_comment || '（无）'}。请修改摘要后重新提交。
        </div>
      )}

      {editable && recording.audio_key && (
        <div className="summary-edit">
          <input
            value={summary}
            placeholder="写一句话摘要"
            onChange={(e) => setSummary(e.target.value)}
          />
          <button className="btn btn-plain btn-small" disabled={saving || !summary.trim()} onClick={saveSummary}>
            保存摘要
          </button>
          <button className="btn btn-primary btn-small" disabled={saving || !summary.trim()} onClick={submit}>
            {recording.status === RECORDING_STATUS_REJECTED ? '重新提交审核' : '提交审核'}
          </button>
        </div>
      )}

      {recording.status === RECORDING_STATUS_APPROVED && (
        <div className="timeline-summary">
          <span className="summary-label">一句话摘要：</span>
          {recording.summary}
        </div>
      )}
    </div>
  )
}

function ReviewerReviewItem({
  recording,
  disabled,
  onNotify,
}: {
  recording: Recording
  disabled: boolean
  onNotify: Notify
}) {
  const { review } = useRecordingStore()
  const [summary, setSummary] = useState(recording.summary)
  const [comment, setComment] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const decide = async (approved: boolean) => {
    if (!approved && !comment.trim()) {
      onNotify('退回时必须填写审核意见', 'error')
      return
    }
    setSubmitting(true)
    try {
      await review(recording.id, {
        approved,
        summary: summary.trim() !== recording.summary ? summary.trim() : undefined,
        comment: approved ? comment.trim() || undefined : comment.trim(),
      })
      onNotify(approved ? '已通过审核，片段进入时间轴' : '已退回采访员')
    } catch (e) {
      onNotify(e instanceof Error ? e.message : '审核操作失败', 'error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="recording-row">
      <RecordingHead recording={recording} />
      {recording.audio_key ? (
        <AudioPlayer recordingId={recording.id} durationSeconds={recording.duration_seconds} />
      ) : (
        <div className="muted">音频尚未上传</div>
      )}

      {recording.status === RECORDING_STATUS_PENDING && !disabled ? (
        <div className="review-panel">
          <textarea value={summary} placeholder="审核时可直接修改一句话摘要" onChange={(e) => setSummary(e.target.value)} />
          <textarea
            value={comment}
            placeholder="审核意见（退回时必填），如：摘要与音频内容不符，请补充时间与人物"
            onChange={(e) => setComment(e.target.value)}
          />
          <div className="review-actions">
            <button className="btn btn-primary btn-small" disabled={submitting || !summary.trim()} onClick={() => decide(true)}>
              ✓ 通过审核
            </button>
            <button className="btn btn-danger btn-small" disabled={submitting || !comment.trim()} onClick={() => decide(false)}>
              ✗ 填写意见后退回
            </button>
          </div>
        </div>
      ) : (
        <div className="timeline-summary">
          <span className="summary-label">一句话摘要：</span>
          {recording.summary || <span className="muted">暂无摘要</span>}
          {recording.status === RECORDING_STATUS_REJECTED && (
            <div className="review-comment">退回意见：{recording.review_comment || '（无）'}</div>
          )}
          {disabled && <div className="review-meta">项目已归档，审核操作已锁定</div>}
        </div>
      )}
    </div>
  )
}

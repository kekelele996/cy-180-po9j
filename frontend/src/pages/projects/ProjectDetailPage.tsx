// 项目详情页：基本信息、采访问题、摘要审核、时间线（仅展示审核通过的录音片段）。
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import AudioPlayer from '../../components/AudioPlayer'
import ConfirmDialog from '../../components/ConfirmDialog'
import EmptyState from '../../components/EmptyState'
import StatusBadge from '../../components/StatusBadge'
import {
  PROJECT_STATUS_ARCHIVED,
  PROJECT_STATUS_COMPLETED,
  PROJECT_STATUS_DRAFT,
  PROJECT_STATUS_IN_PROGRESS,
  REVIEW_STATUS_APPROVED,
  REVIEW_STATUS_DRAFT,
  REVIEW_STATUS_PENDING,
  REVIEW_STATUS_REJECTED,
  ROLE_ADMIN,
  ROLE_ARCHIVIST,
  ROLE_INTERVIEWER,
} from '../../constants'
import { useAuthStore } from '../../stores/authStore'
import { useProjectStore } from '../../stores/projectStore'
import { useQuestionStore } from '../../stores/questionStore'
import { useRecordingStore } from '../../stores/recordingStore'
import { useTimelineStore } from '../../stores/timelineStore'
import { formatDateTime, formatDuration } from '../../utils/format'
import type { Recording, TimelineMarker } from '../../api/types'

export default function ProjectDetailPage() {
  const { id } = useParams()
  const projectId = Number(id)
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const { detail, fetchDetail, transitionStatus, remove } = useProjectStore()
  const { questions, fetchByProject, create: createQuestion, remove: removeQuestion } = useQuestionStore()
  const { recordings, fetchByProject: fetchRecordings } = useRecordingStore()
  const { markers, fetchByProject: fetchMarkers, create: createMarker } = useTimelineStore()
  const [newQuestion, setNewQuestion] = useState('')
  const [message, setMessage] = useState<{ text: string; kind: 'success' | 'error' } | null>(null)

  const showMessage = useCallback((text: string, kind: 'success' | 'error' = 'success') => {
    setMessage({ text, kind })
    setTimeout(() => setMessage(null), 3000)
  }, [])

  useEffect(() => {
    if (projectId) {
      fetchDetail(projectId)
      fetchByProject(projectId)
      fetchRecordings(projectId)
      fetchMarkers(projectId)
    }
  }, [projectId, fetchDetail, fetchByProject, fetchRecordings, fetchMarkers])

  const handleAddQuestion = useCallback(async () => {
    if (!newQuestion.trim()) return
    await createQuestion(projectId, newQuestion.trim())
    setNewQuestion('')
    showMessage('采访问题已添加')
  }, [createQuestion, newQuestion, projectId, showMessage])

  const markersOf = (recordingId: number) => markers.filter((m) => m.recording_id === recordingId)

  // 时间线只呈现审核通过的片段。
  const approvedRecordings = recordings.filter((r) => r.review_status === REVIEW_STATUS_APPROVED)
  const reviewRecordings = recordings.filter((r) => r.audio_key)
  const pendingCount = recordings.filter((r) => r.review_status === REVIEW_STATUS_PENDING).length
  const rejectedCount = recordings.filter((r) => r.review_status === REVIEW_STATUS_REJECTED).length

  if (!detail) {
    return <div className="page">加载中…</div>
  }

  const isArchived = detail.status === PROJECT_STATUS_ARCHIVED

  const handleArchive = async () => {
    try {
      await transitionStatus(projectId, PROJECT_STATUS_ARCHIVED)
      showMessage('项目已归档')
    } catch (e) {
      showMessage(e instanceof Error ? e.message : '归档失败', 'error')
    }
  }

  return (
    <div className="page">
      {message && <div className={`toast ${message.kind}`}>{message.text}</div>}
      <div className="page-header">
        <button className="btn btn-plain" onClick={() => navigate('/')}>
          ← 返回列表
        </button>
        <h2>{detail.title}</h2>
        <StatusBadge status={detail.status} type="project" />
      </div>

      <section className="card">
        <div className="card-title">项目信息</div>
        <div className="detail-grid">
          <div>
            <div className="detail-label">受访者</div>
            <div className="detail-value">{detail.interviewee_name}</div>
          </div>
          <div>
            <div className="detail-label">出生年份</div>
            <div className="detail-value">{detail.birth_year}</div>
          </div>
          <div>
            <div className="detail-label">创建时间</div>
            <div className="detail-value">{formatDateTime(detail.created_at)}</div>
          </div>
          <div>
            <div className="detail-label">背景简介</div>
            <div className="detail-value">{detail.background || '-'}</div>
          </div>
        </div>
        <div className="row-actions" style={{ marginTop: 12 }}>
          {detail.status === PROJECT_STATUS_IN_PROGRESS && (
            <button
              className="btn btn-primary btn-small"
              onClick={async () => {
                await transitionStatus(projectId, PROJECT_STATUS_COMPLETED)
                showMessage('项目状态已更新')
              }}
            >
              标记为已完成
            </button>
          )}
          {detail.status === PROJECT_STATUS_COMPLETED && (
            <>
              <button className="btn btn-primary btn-small" onClick={handleArchive}>
                归档项目
              </button>
              <button
                className="btn btn-plain btn-small"
                onClick={async () => {
                  await transitionStatus(projectId, PROJECT_STATUS_IN_PROGRESS)
                  showMessage('项目状态已更新')
                }}
              >
                退回为进行中
              </button>
            </>
          )}
          {detail.status === PROJECT_STATUS_DRAFT && (
            <button
              className="btn btn-primary btn-small"
              onClick={async () => {
                await transitionStatus(projectId, PROJECT_STATUS_IN_PROGRESS)
                showMessage('项目状态已更新')
              }}
            >
              开始采访
            </button>
          )}
          <ConfirmDialog
            title="删除采访项目"
            message="确定删除该项目吗？此操作不可恢复。"
            danger
            confirmText="删除"
            onConfirm={async () => {
              await remove(projectId)
              navigate('/')
            }}
          >
            <button className="btn btn-danger btn-small">删除项目</button>
          </ConfirmDialog>
          <LinkToInterview projectId={projectId} />
        </div>
        {(pendingCount > 0 || rejectedCount > 0) && (
          <div className="review-banner">
            归档前需处理完所有摘要审核：当前 {pendingCount} 条待审核、{rejectedCount} 条已退回。
          </div>
        )}
      </section>

      <section className="card">
        <div className="card-title">采访问题</div>
        {questions.length === 0 ? (
          <EmptyState title="还没有采访问题" description="添加采访问题，作为录音的提纲" />
        ) : (
          <ul className="question-list">
            {questions.map((q) => (
              <li key={q.id} className="question-item">
                <span className="question-index">{q.sort_order + 1}</span>
                <span className="question-content">{q.content}</span>
                <ConfirmDialog
                  title="删除采访问题"
                  message="删除问题将同时删除其下的录音片段，确定继续？"
                  danger
                  confirmText="删除"
                  onConfirm={() => removeQuestion(q.id)}
                >
                  <button className="btn btn-plain btn-small">删除</button>
                </ConfirmDialog>
              </li>
            ))}
          </ul>
        )}
        <div className="inline-form">
          <input value={newQuestion} onChange={(e) => setNewQuestion(e.target.value)} placeholder="输入新的采访问题" />
          <button className="btn btn-primary" onClick={handleAddQuestion} disabled={!newQuestion.trim()}>
            添加问题
          </button>
        </div>
      </section>

      <section className="card">
        <div className="card-title">摘要审核</div>
        {reviewRecordings.length === 0 ? (
          <EmptyState title="还没有可审核的录音" description="前往采访工作台录制并上传音频后，在此提交摘要审核" />
        ) : (
          <div className="review-list">
            {reviewRecordings.map((r) => (
              <ReviewItem
                key={r.id}
                recording={r}
                questionContent={questions.find((q) => q.id === r.question_id)?.content}
                role={user?.role || ''}
                archived={isArchived}
                onChanged={showMessage}
                onRefresh={() => fetchRecordings(projectId)}
              />
            ))}
          </div>
        )}
      </section>

      <section className="card">
        <div className="card-title">时间线 · 采访片段</div>
        {approvedRecordings.length === 0 ? (
          <EmptyState
            title="时间线上还没有片段"
            description="只有摘要审核通过的片段才会出现在项目时间线上"
          />
        ) : (
          <div className="timeline">
            {approvedRecordings.map((r) => (
              <TimelineItem key={r.id} recording={r} markers={markersOf(r.id)} onCreateMarker={createMarker} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function LinkToInterview({ projectId }: { projectId: number }) {
  return (
    <a className="btn btn-plain btn-small" href={`#/interview?project_id=${projectId}`}>
      前往采访工作台
    </a>
  )
}

function ReviewItem({
  recording,
  questionContent,
  role,
  archived,
  onChanged,
  onRefresh,
}: {
  recording: Recording
  questionContent?: string
  role: string
  archived: boolean
  onChanged: (text: string, kind?: 'success' | 'error') => void
  onRefresh: () => Promise<void>
}) {
  const { updateSummary, submitForReview, approveReview, rejectReview } = useRecordingStore()
  const [summaryDraft, setSummaryDraft] = useState(recording.summary)
  const [commentDraft, setCommentDraft] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setSummaryDraft(recording.summary)
  }, [recording.summary])

  const run = async (action: () => Promise<void>, okText: string) => {
    setSaving(true)
    try {
      await action()
      await onRefresh()
      onChanged(okText)
    } catch (e) {
      onChanged(e instanceof Error ? e.message : '操作失败', 'error')
    } finally {
      setSaving(false)
    }
  }
  const isInterviewer = role === ROLE_INTERVIEWER || role === ROLE_ADMIN
  const isReviewer = role === ROLE_ARCHIVIST || role === ROLE_ADMIN
  const summaryEditable =
    !archived &&
    ((recording.review_status === REVIEW_STATUS_DRAFT || recording.review_status === REVIEW_STATUS_REJECTED) &&
      isInterviewer ||
      (recording.review_status === REVIEW_STATUS_PENDING && isReviewer))

  return (
    <div className="review-item">
      <div className="review-head">
        <span className="timeline-q">
          {questionContent ? `问题：${questionContent}` : `问题 #${recording.question_id}`}
        </span>
        <StatusBadge status={recording.review_status} type="review" />
        <span className="timeline-duration">{formatDuration(recording.duration_seconds)}</span>
      </div>
      <AudioPlayer recordingId={recording.id} durationSeconds={recording.duration_seconds} />
      <div className="review-summary-row">
        <input
          value={summaryDraft}
          placeholder="写一句话摘要"
          onChange={(e) => setSummaryDraft(e.target.value)}
          disabled={!summaryEditable}
        />
      </div>
      {recording.review_status === REVIEW_STATUS_REJECTED && recording.review_comment && (
        <div className="review-comment">退回意见：{recording.review_comment}</div>
      )}
      {recording.review_status === REVIEW_STATUS_APPROVED && recording.reviewed_at && (
        <div className="muted review-meta">审核通过时间：{formatDateTime(recording.reviewed_at)}</div>
      )}

      <div className="row-actions">
        {/* 采访员：保存摘要 + 提交审核 */}
        {!archived &&
          isInterviewer &&
          (recording.review_status === REVIEW_STATUS_DRAFT || recording.review_status === REVIEW_STATUS_REJECTED) && (
            <>
              <button
                className="btn btn-plain btn-small"
                disabled={saving || !summaryDraft.trim() || summaryDraft.trim() === recording.summary}
                onClick={() => run(() => updateSummary(recording.id, summaryDraft.trim()), '摘要已保存')}
              >
                保存摘要
              </button>
              <button
                className="btn btn-primary btn-small"
                disabled={saving || !summaryDraft.trim()}
                onClick={() => run(() => submitForReview(recording.id), '已提交审核')}
              >
                提交审核
              </button>
            </>
          )}
        {/* 采访员视角：提交后等待审核（管理员走审核区，不显示等待提示） */}
        {!archived && recording.review_status === REVIEW_STATUS_PENDING && role === ROLE_INTERVIEWER && (
          <span className="muted">已提交，等待档案员审核…</span>
        )}

        {/* 档案员/管理员：审核通过（可改摘要）或退回（填意见） */}
        {!archived && recording.review_status === REVIEW_STATUS_PENDING && isReviewer && (
          <>
            <button
              className="btn btn-primary btn-small"
              disabled={saving}
              onClick={() =>
                run(
                  () =>
                    approveReview(
                      recording.id,
                      summaryDraft.trim() && summaryDraft.trim() !== recording.summary
                        ? summaryDraft.trim()
                        : undefined,
                    ),
                  '已审核通过',
                )
              }
            >
              审核通过
            </button>
            <input
              className="reject-input"
              value={commentDraft}
              placeholder="填写退回意见（必填）"
              onChange={(e) => setCommentDraft(e.target.value)}
            />
            <button
              className="btn btn-danger btn-small"
              disabled={saving || !commentDraft.trim()}
              onClick={async () => {
                await run(() => rejectReview(recording.id, commentDraft.trim()), '已退回修改')
                setCommentDraft('')
              }}
            >
              退回修改
            </button>
          </>
        )}
      </div>
    </div>
  )
}

function TimelineItem({
  recording,
  markers,
  onCreateMarker,
}: {
  recording: Recording
  markers: TimelineMarker[]
  onCreateMarker: (payload: {
    project_id: number
    recording_id: number
    timestamp_second: number
    label: string
    note?: string
  }) => Promise<void>
}) {
  const [label, setLabel] = useState('')
  const question = useQuestionStore((s) => s.questions.find((q) => q.id === recording.question_id))

  return (
    <div className="timeline-item">
      <div className="timeline-dot" />
      <div className="timeline-content">
        <div className="timeline-head">
          <span className="timeline-q">{question ? `问题：${question.content}` : `问题 #${recording.question_id}`}</span>
          <StatusBadge status={recording.status} type="recording" />
          <span className="timeline-duration">{formatDuration(recording.duration_seconds)}</span>
        </div>
        <AudioPlayer recordingId={recording.id} durationSeconds={recording.duration_seconds} />
        <div className="timeline-summary">
          <span className="summary-label">一句话摘要：</span>
          {recording.summary || <span className="muted">暂无摘要</span>}
        </div>
        {markers.length > 0 && (
          <div className="marker-list">
            {markers.map((m) => (
              <span key={m.id} className="marker-chip">
                ⏱ {formatDuration(m.timestamp_second)} · {m.label}
                {m.note ? `（${m.note}）` : ''}
              </span>
            ))}
          </div>
        )}
        <div className="inline-form">
          <input
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="标注关键节点，如：回忆童年故居"
          />
          <button
            className="btn btn-plain btn-small"
            disabled={!label.trim()}
            onClick={async () => {
              await onCreateMarker({
                project_id: recording.project_id,
                recording_id: recording.id,
                timestamp_second: recording.duration_seconds > 0 ? Math.floor(recording.duration_seconds / 2) : 0,
                label: label.trim(),
              })
              setLabel('')
            }}
          >
            ＋ 标注节点
          </button>
        </div>
      </div>
    </div>
  )
}

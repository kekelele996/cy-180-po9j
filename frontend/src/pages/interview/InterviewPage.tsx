// 采访工作台：选择项目 → 按问题录音 → 自动关联 → 一句话摘要 → 提交摘要审核。
import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import AudioPlayer from '../../components/AudioPlayer'
import EmptyState from '../../components/EmptyState'
import StatusBadge from '../../components/StatusBadge'
import {
  RECORDING_STATUS_APPROVED,
  RECORDING_STATUS_PENDING,
  RECORDING_STATUS_READY,
  RECORDING_STATUS_REJECTED,
} from '../../constants'
import { useProjectStore } from '../../stores/projectStore'
import { useQuestionStore } from '../../stores/questionStore'
import { useRecordingStore } from '../../stores/recordingStore'
import { useTimelineStore } from '../../stores/timelineStore'
import { formatDuration } from '../../utils/format'
import type { Recording } from '../../api/types'

export default function InterviewPage() {
  const [params, setParams] = useSearchParams()
  const selectedProject = Number(params.get('project_id')) || 0
  const { projects, fetchList } = useProjectStore()
  const { questions, fetchByProject } = useQuestionStore()
  const { fetchByProject: fetchRecordings } = useRecordingStore()
  const [activeQuestion, setActiveQuestion] = useState(0)
  const [message, setMessage] = useState('')

  useEffect(() => {
    fetchList({ page: 1, page_size: 100 })
  }, [fetchList])

  useEffect(() => {
    if (selectedProject) {
      fetchByProject(selectedProject)
      fetchRecordings(selectedProject)
      setActiveQuestion(0)
    } else {
      setActiveQuestion(0)
    }
  }, [selectedProject, fetchByProject, fetchRecordings])

  const chooseProject = (projectId: number) => {
    const next = new URLSearchParams(params)
    if (projectId) {
      next.set('project_id', String(projectId))
    } else {
      next.delete('project_id')
    }
    setParams(next)
  }

  return (
    <div className="page">
      <div className="page-header">
        <h2>采访工作台</h2>
      </div>
      {message && <div className="toast success">{message}</div>}

      <section className="card">
        <div className="card-title">选择采访项目</div>
        <select value={selectedProject} onChange={(e) => chooseProject(Number(e.target.value))}>
          <option value={0}>请选择项目</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.title}（{p.interviewee_name}）
            </option>
          ))}
        </select>
      </section>

      {selectedProject === 0 ? (
        <EmptyState title="请先选择采访项目" description="选择一个项目后即可开始录音" />
      ) : (
        <>
          <section className="card">
            <div className="card-title">采访问题列表</div>
            {questions.length === 0 ? (
              <EmptyState title="该项目还没有采访问题" description="请先到项目详情页添加采访问题" />
            ) : (
              <div className="question-tabs">
                {questions.map((q, idx) => (
                  <button
                    key={q.id}
                    className={`question-tab ${activeQuestion === q.id ? 'active' : ''}`}
                    onClick={() => setActiveQuestion(q.id)}
                  >
                    {idx + 1}. {q.content}
                  </button>
                ))}
              </div>
            )}
          </section>

          {activeQuestion > 0 && (
            <RecorderPanel
              projectId={selectedProject}
              questionId={activeQuestion}
              onRecorded={(summary) => {
                fetchRecordings(selectedProject)
                setMessage(summary)
                setTimeout(() => setMessage(''), 4000)
              }}
            />
          )}
        </>
      )}
    </div>
  )
}

function RecorderPanel({
  projectId,
  questionId,
  onRecorded,
}: {
  projectId: number
  questionId: number
  onRecorded: (msg: string) => void
}) {
  const { create, uploadAudio, fetchByQuestion } = useRecordingStore()
  const [recording, setRecording] = useState(false)
  const [seconds, setSeconds] = useState(0)
  const [uploading, setUploading] = useState(0)
  const [recordings, setRecordings] = useState<Awaited<ReturnType<typeof fetchByQuestion>>>([])
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const timerRef = useRef<number | null>(null)

  const reload = useCallback(async () => {
    setRecordings(await fetchByQuestion(questionId))
  }, [fetchByQuestion, questionId])

  useEffect(() => {
    reload()
  }, [reload])

  useEffect(() => {
    if (recording) {
      timerRef.current = window.setInterval(() => setSeconds((s) => s + 1), 1000)
    } else if (timerRef.current) {
      window.clearInterval(timerRef.current)
      timerRef.current = null
    }
    return () => {
      if (timerRef.current) window.clearInterval(timerRef.current)
    }
  }, [recording])

  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const recorder = new MediaRecorder(stream)
      chunksRef.current = []
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) chunksRef.current.push(e.data)
      }
      recorder.onstop = async () => {
        stream.getTracks().forEach((t) => t.stop())
        const blob = new Blob(chunksRef.current, { type: 'audio/webm' })
        setRecording(false)
        setUploading(1)
        try {
          const created = await create({ project_id: projectId, question_id: questionId, duration_seconds: seconds })
          await uploadAudio(created.id, blob, seconds, (p) => setUploading(p))
          await reload()
          onRecorded('录音上传成功，已自动关联到当前问题')
        } catch (e) {
          onRecorded(e instanceof Error ? e.message : '录音上传失败')
        } finally {
          setUploading(0)
          setSeconds(0)
        }
      }
      mediaRecorderRef.current = recorder
      recorder.start()
      setRecording(true)
      setSeconds(0)
    } catch {
      alert('无法访问麦克风，请在浏览器中授权麦克风权限')
    }
  }

  const stopRecording = () => {
    mediaRecorderRef.current?.stop()
  }

  return (
    <section className="card">
      <div className="card-title">录音面板</div>
      <div className="recorder-box">
        {uploading > 0 ? (
          <div className="upload-progress">
            上传中… {uploading}%
            <div className="progress-bar">
              <div className="progress-inner" style={{ width: `${uploading}%` }} />
            </div>
          </div>
        ) : recording ? (
          <>
            <div className="recording-indicator">
              <span className="rec-dot" /> 正在录音 {formatDuration(seconds)}
            </div>
            <button className="btn btn-danger" onClick={stopRecording}>
              ⏹ 停止并保存
            </button>
          </>
        ) : (
          <button className="btn btn-primary" onClick={startRecording}>
            ⏺ 开始录音
          </button>
        )}
      </div>

      <div className="card-title" style={{ marginTop: 20 }}>
        本问题已录片段（{recordings.length}）
      </div>
      {recordings.length === 0 ? (
        <EmptyState title="还没有录音" description="点击上方开始录音" />
      ) : (
        <div className="recording-list">
          {recordings.map((r) => (
            <InterviewRecordingRow
              key={r.id}
              recording={r}
              projectId={projectId}
              onChanged={reload}
              onMessage={onRecorded}
            />
          ))}
        </div>
      )}
    </section>
  )
}

function InterviewRecordingRow({
  recording,
  projectId,
  onChanged,
  onMessage,
}: {
  recording: Recording
  projectId: number
  onChanged: () => Promise<void>
  onMessage: (msg: string) => void
}) {
  const { updateSummary, submitForReview } = useRecordingStore()
  const { markers, fetchByRecording, create: createMarker } = useTimelineStore()
  const [summary, setSummary] = useState(recording.summary)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setSummary(recording.summary)
  }, [recording.summary])

  useEffect(() => {
    fetchByRecording(recording.id)
  }, [fetchByRecording, recording.id])

  const editable = [RECORDING_STATUS_READY, RECORDING_STATUS_REJECTED].includes(recording.status)

  const saveSummary = async () => {
    if (!summary.trim()) {
      onMessage('请先填写一句话摘要')
      return
    }
    setBusy(true)
    try {
      await updateSummary(recording.id, summary.trim())
      await onChanged()
      onMessage('摘要已保存')
    } catch (e) {
      onMessage(e instanceof Error ? e.message : '摘要保存失败')
    } finally {
      setBusy(false)
    }
  }

  const submit = async () => {
    if (!recording.audio_key) {
      onMessage('音频尚未上传，不能提交审核')
      return
    }
    if (!summary.trim()) {
      onMessage('请先填写一句话摘要再提交')
      return
    }
    setBusy(true)
    try {
      if (summary.trim() !== recording.summary) {
        await updateSummary(recording.id, summary.trim())
      }
      await submitForReview(recording.id)
      await onChanged()
      onMessage(recording.status === RECORDING_STATUS_REJECTED ? '已重新提交审核' : '已提交审核，等待档案员处理')
    } catch (e) {
      onMessage(e instanceof Error ? e.message : '提交审核失败')
    } finally {
      setBusy(false)
    }
  }

  const addMarker = async () => {
    const input = document.getElementById(`marker-input-${recording.id}`) as HTMLInputElement | null
    const label = input?.value.trim()
    if (!input || !label) return
    try {
      await createMarker({ project_id: projectId, recording_id: recording.id, timestamp_second: 0, label })
      input.value = ''
      await fetchByRecording(recording.id)
    } catch (e) {
      onMessage(e instanceof Error ? e.message : '节点标注失败')
    }
  }

  return (
    <div className="recording-row">
      <div className="recording-meta">
        <StatusBadge status={recording.status} type="recording" />
        <span className="muted">{formatDuration(recording.duration_seconds)}</span>
      </div>
      {recording.audio_key ? (
        <AudioPlayer recordingId={recording.id} durationSeconds={recording.duration_seconds} />
      ) : (
        <div className="muted">音频尚未上传</div>
      )}

      {recording.status === RECORDING_STATUS_REJECTED && (
        <div className="review-comment">审核退回意见：{recording.review_comment || '（无）'}，请修改后重新提交</div>
      )}
      {recording.status === RECORDING_STATUS_PENDING && (
        <div className="timeline-summary muted">摘要已提交，正在等待档案员或管理员审核</div>
      )}
      {recording.status === RECORDING_STATUS_APPROVED && (
        <div className="timeline-summary">
          <span className="summary-label muted">已通过审核的摘要：</span>
          {recording.summary}
        </div>
      )}

      {editable && recording.audio_key && (
        <div className="summary-edit">
          <input value={summary} placeholder="写一句话摘要" onChange={(e) => setSummary(e.target.value)} />
          <button className="btn btn-plain btn-small" disabled={busy || !summary.trim()} onClick={saveSummary}>
            保存摘要
          </button>
          <button className="btn btn-primary btn-small" disabled={busy || !summary.trim()} onClick={submit}>
            {recording.status === RECORDING_STATUS_REJECTED ? '重新提交审核' : '提交审核'}
          </button>
        </div>
      )}
      <div className="marker-actions">
        <span className="muted">时间轴节点：</span>
        {markers
          .filter((m) => m.recording_id === recording.id)
          .map((m) => (
            <span key={m.id} className="marker-chip">
              {m.label}
            </span>
          ))}
        <input placeholder="新增节点，如：讲到参军经历" style={{ maxWidth: 220 }} id={`marker-input-${recording.id}`} />
        <button className="btn btn-plain btn-small" onClick={addMarker}>
          ＋ 标注
        </button>
      </div>
    </div>
  )
}

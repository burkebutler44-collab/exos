export function ConfirmDialog({
  open,
  title,
  detail,
  reason,
  onReasonChange,
  onCancel,
  onConfirm,
}: {
  open: boolean
  title: string
  detail: string
  reason: string
  onReasonChange: (value: string) => void
  onCancel: () => void
  onConfirm: () => void
}) {
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
      <div className="w-full max-w-md rounded-lg border border-line bg-white shadow-xl">
        <div className="border-b border-line px-4 py-3">
          <h2 className="text-[15px] font-semibold text-ink">{title}</h2>
          <p className="mt-1 text-[12.5px] text-muted">{detail}</p>
        </div>
        <div className="p-4">
          <label className="admin-label" htmlFor="confirm-reason">Required reason</label>
          <textarea id="confirm-reason" className="admin-input mt-2 min-h-24" value={reason} onChange={(event) => onReasonChange(event.target.value)} />
        </div>
        <div className="flex justify-end gap-2 border-t border-line px-4 py-3">
          <button type="button" className="admin-btn" onClick={onCancel}>Cancel</button>
          <button type="button" className="admin-btn-primary disabled:opacity-50" disabled={!reason.trim()} onClick={onConfirm}>Confirm</button>
        </div>
      </div>
    </div>
  )
}

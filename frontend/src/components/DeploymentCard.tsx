import type { Deployment } from "../services/api";

type DeploymentCardProps = {
  deployment: Deployment;
  onViewLogs?: (id: string) => void;
  onDelete?: (id: string) => void;
};

const statusColors: Record<string, string> = {
  running: "bg-emerald-100 text-emerald-800 border border-emerald-200",
  building: "bg-amber-100 text-amber-800 border border-amber-200",
  cloning: "bg-primary-100 text-primary-800 border border-primary-200",
  starting: "bg-cyan-100 text-cyan-800 border border-cyan-200",
  queued: "bg-slate-100 text-slate-700 border border-slate-200",
  stopped: "bg-slate-100 text-slate-700 border border-slate-200",
  failed: "bg-red-100 text-red-800 border border-red-200",
};

const statusDots: Record<string, string> = {
  running: "bg-emerald-500",
  building: "bg-amber-500 animate-pulse",
  cloning: "bg-primary-500 animate-pulse",
  starting: "bg-cyan-500 animate-pulse",
  queued: "bg-slate-400 animate-pulse",
  stopped: "bg-slate-400",
  failed: "bg-red-500",
};

export default function DeploymentCard({ deployment, onViewLogs, onDelete }: DeploymentCardProps) {
  const colorClass = statusColors[deployment.status] ?? "bg-slate-100 text-slate-700 border border-slate-200";
  const dotClass = statusDots[deployment.status] ?? "bg-slate-400";

  return (
    <article className="surface-card p-4 sm:p-5 hover:shadow-glow transition-all duration-300 hover:-translate-y-0.5 reveal-up">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2 min-w-0">
          <span className={`w-2.5 h-2.5 rounded-full ${dotClass} shrink-0 mt-0.5`} />
          <strong className="text-slate-900 font-semibold truncate">{deployment.repository}</strong>
        </div>
        <span className={`px-2.5 py-1 rounded-full text-xs font-medium shrink-0 ml-2 ${colorClass}`}>
          {deployment.status}
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500 mb-3">
        <span>
          Branch: <span className="font-semibold text-slate-700">{deployment.branch}</span>
        </span>
        <span>{new Date(deployment.createdAt).toLocaleDateString()}</span>
      </div>

      {deployment.url && deployment.url !== "about:blank" && (
        <a
          href={deployment.url}
          target="_blank"
          rel="noreferrer"
          className="text-xs text-primary-700 hover:text-primary-800 hover:underline truncate block"
        >
          {deployment.url}
        </a>
      )}
      {deployment.localUrl && deployment.localUrl !== deployment.url && (
        <a
          href={deployment.localUrl}
          target="_blank"
          rel="noreferrer"
          className="text-xs text-cyan-700 hover:text-cyan-800 hover:underline truncate block mt-1"
        >
          Local: {deployment.localUrl}
        </a>
      )}
      {deployment.error && <p className="text-xs text-red-700 mt-2 line-clamp-2">{deployment.error}</p>}

      <div className="flex items-center gap-2 mt-4">
        <button
          type="button"
          onClick={() => onViewLogs?.(deployment.id)}
          aria-label={`View logs for ${deployment.repository}`}
          className="focus-ring min-h-9 text-xs px-3 py-1.5 rounded-lg border border-slate-300 text-slate-700 hover:bg-slate-100 transition"
        >
          Logs
        </button>
        <button
          type="button"
          onClick={() => onDelete?.(deployment.id)}
          aria-label={`Delete deployment ${deployment.repository}`}
          className="focus-ring min-h-9 text-xs px-3 py-1.5 rounded-lg border border-red-300 text-red-700 hover:bg-red-50 transition"
        >
          Delete
        </button>
      </div>
    </article>
  );
}

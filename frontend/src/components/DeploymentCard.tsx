import type { Deployment } from "../services/api";

type DeploymentCardProps = {
  deployment: Deployment;
  onViewLogs?: (id: string) => void;
  onDelete?: (id: string) => void;
};

const statusColors: Record<string, string> = {
  running:  "bg-green-100 text-green-800",
  building: "bg-yellow-100 text-yellow-800",
  stopped:  "bg-gray-100 text-gray-700",
  failed:   "bg-red-100 text-red-800",
};

const statusDots: Record<string, string> = {
  running:  "bg-green-500",
  building: "bg-yellow-500 animate-pulse",
  stopped:  "bg-gray-400",
  failed:   "bg-red-500",
};

export default function DeploymentCard({ deployment, onViewLogs, onDelete }: DeploymentCardProps) {
  const colorClass = statusColors[deployment.status] ?? "bg-gray-100 text-gray-700";
  const dotClass   = statusDots[deployment.status]   ?? "bg-gray-400";

  return (
    <article className="border border-gray-100 rounded-xl p-5 hover:shadow-md transition-shadow bg-gray-50/50">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <span className={`w-2.5 h-2.5 rounded-full ${dotClass} mt-0.5 shrink-0`} />
          <strong className="text-gray-900 font-semibold truncate">{deployment.repository}</strong>
        </div>
        <span className={`px-2.5 py-1 rounded-full text-xs font-medium shrink-0 ml-2 ${colorClass}`}>
          {deployment.status}
        </span>
      </div>

      <div className="flex items-center gap-4 text-xs text-gray-500 mb-3">
        <span>
          Branch: <span className="font-medium text-gray-700">{deployment.branch}</span>
        </span>
        <span>{new Date(deployment.createdAt).toLocaleDateString()}</span>
      </div>

      {deployment.url && (
        <a
          href={deployment.url}
          target="_blank"
          rel="noreferrer"
          className="text-xs text-primary-600 hover:text-primary-700 hover:underline truncate block"
        >
          {deployment.url}
        </a>
      )}
      {deployment.localUrl && deployment.localUrl !== deployment.url ? (
        <a
          href={deployment.localUrl}
          target="_blank"
          rel="noreferrer"
          className="text-xs text-blue-600 hover:text-blue-700 hover:underline truncate block mt-1"
        >
          Local URL: {deployment.localUrl}
        </a>
      ) : null}
      {deployment.error ? <p className="text-xs text-red-600 mt-2">{deployment.error}</p> : null}

      <div className="flex items-center gap-2 mt-3">
        <button
          type="button"
          onClick={() => onViewLogs?.(deployment.id)}
          className="text-xs px-2 py-1 rounded border border-gray-300 text-gray-700 hover:bg-gray-100"
        >
          Logs
        </button>
        <button
          type="button"
          onClick={() => onDelete?.(deployment.id)}
          className="text-xs px-2 py-1 rounded border border-red-300 text-red-700 hover:bg-red-50"
        >
          Delete
        </button>
      </div>
    </article>
  );
}

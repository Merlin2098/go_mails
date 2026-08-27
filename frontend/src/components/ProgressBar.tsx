import { formatElapsed, useElapsedTime } from "../hooks/useElapsedTime";

interface ProgressBarProps {
  processed: number;
  total: number;
  running: boolean;
}

export default function ProgressBar({ processed, total, running }: ProgressBarProps) {
  const percent = total === 0 ? 0 : Math.round((processed / total) * 100);
  const elapsedSeconds = useElapsedTime(running);

  return (
    <div className="progress">
      <div className="progress-track">
        <div className="progress-fill" style={{ width: `${percent}%` }} />
      </div>
      <span className="progress-label">
        {processed} / {total} ({percent}%) · {formatElapsed(elapsedSeconds)}
      </span>
    </div>
  );
}

import {
  Clock,
  CheckCircle2,
  XCircle,
  Loader2,
  Play,
  PauseCircle,
} from "lucide-react";

export const taskStatusConfig: Record<string, { label: string; icon: typeof CheckCircle2; color: string }> = {
  queued: { label: "Queued", icon: Clock, color: "text-muted-foreground" },
  dispatched: { label: "Dispatched", icon: Play, color: "text-info" },
  waiting_local_directory: { label: "Waiting", icon: PauseCircle, color: "text-warning" },
  running: { label: "Running", icon: Loader2, color: "text-brand" },
  completed: { label: "Completed", icon: CheckCircle2, color: "text-success" },
  failed: { label: "Failed", icon: XCircle, color: "text-destructive" },
  cancelled: { label: "Cancelled", icon: XCircle, color: "text-muted-foreground" },
};

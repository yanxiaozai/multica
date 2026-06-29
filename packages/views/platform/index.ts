export { useImmersiveMode } from "./use-immersive-mode";
export { useDesktopUnreadBadge } from "./use-desktop-unread-badge";
export { DragStrip } from "./drag-strip";
export { openExternal } from "./open-external";
export {
  isDesktopShell,
  pickDirectory,
  validateLocalDirectory,
  detectGitRemote,
  parseGitRemote,
  type PickDirectoryResult,
  type ValidateLocalDirectoryResult,
  type DetectGitRemoteResult,
  type ParsedGitRemote,
} from "./local-directory";
export {
  useLocalDaemonStatus,
  type LocalDaemonStatus,
} from "./use-local-daemon-status";
export {
  isClaudeAgentImportSupported,
  listClaudeAgentFiles,
  parseClaudeAgentFile,
  type ClaudeAgentFile,
  type ClaudeAgentDefinition,
} from "./claude-agents";

"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Loader2, Save } from "lucide-react";
import type { Agent } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { toast } from "sonner";
import { useT } from "../../../i18n";

function profileToText(value: unknown): string {
  if (value === null || value === undefined) return "{}";
  return JSON.stringify(value, null, 2);
}

export function SpecProfileTab({
  agent,
  onSave,
  onDirtyChange,
}: {
  agent: Agent;
  onSave: (updates: { spec_profile: Record<string, unknown> }) => Promise<void>;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const { t } = useT("agents");
  const original = useMemo(() => profileToText(agent.spec_profile ?? {}), [agent.spec_profile]);
  const [text, setText] = useState(original);
  const [saving, setSaving] = useState(false);

  const previousOriginalRef = useRef(original);
  useEffect(() => {
    setText((current) =>
      current === previousOriginalRef.current ? original : current,
    );
    previousOriginalRef.current = original;
  }, [original]);

  const trimmed = text.trim();
  const parseResult = useMemo<
    | { ok: true; value: Record<string, unknown> }
    | { ok: false; error: string }
  >(() => {
    if (trimmed === "") return { ok: true, value: {} };
    try {
      const value = JSON.parse(trimmed);
      if (value === null || typeof value !== "object" || Array.isArray(value)) {
        return { ok: false, error: "spec_profile_not_object" };
      }
      return { ok: true, value: value as Record<string, unknown> };
    } catch (err) {
      return {
        ok: false,
        error: err instanceof Error ? err.message : "invalid JSON",
      };
    }
  }, [trimmed]);

  const dirty = text !== original;

  useEffect(() => {
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);

  const handleSave = async () => {
    if (!parseResult.ok) return;
    setSaving(true);
    try {
      await onSave({ spec_profile: parseResult.value });
      setText(profileToText(parseResult.value));
      toast.success(t(($) => $.tab_body.spec_profile.saved_toast));
    } catch (err) {
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : t(($) => $.tab_body.spec_profile.save_failed_toast),
      );
    } finally {
      setSaving(false);
    }
  };

  const showInvalid = trimmed !== "" && !parseResult.ok;
  const invalidMessage = !parseResult.ok && parseResult.error === "spec_profile_not_object"
    ? t(($) => $.tab_body.spec_profile.invalid_not_object)
    : !parseResult.ok
      ? t(($) => $.tab_body.spec_profile.invalid_json, { error: parseResult.error })
      : "";

  return (
    <div className="flex h-full flex-col space-y-3">
      <p className="text-xs text-muted-foreground">
        {t(($) => $.tab_body.spec_profile.intro)}
      </p>

      <Textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={t(($) => $.tab_body.spec_profile.placeholder)}
        aria-invalid={showInvalid || undefined}
        aria-label={t(($) => $.tab_body.spec_profile.editor_aria)}
        spellCheck={false}
        className="min-h-[240px] flex-1 font-mono text-xs"
      />

      {showInvalid && (
        <p className="text-xs text-destructive">{invalidMessage}</p>
      )}

      <div className="flex items-center justify-end gap-3">
        {dirty && (
          <span className="text-xs text-muted-foreground">
            {t(($) => $.tab_body.common.unsaved_changes)}
          </span>
        )}
        <Button
          onClick={handleSave}
          disabled={!dirty || !parseResult.ok || saving}
          size="sm"
        >
          {saving ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Save className="h-3.5 w-3.5" />
          )}
          {t(($) => $.tab_body.common.save)}
        </Button>
      </div>
    </div>
  );
}

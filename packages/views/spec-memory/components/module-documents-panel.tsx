"use client";

import { useEffect, useMemo, useState } from "react";
import { FileText, Save } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import {
  specDocumentListOptions,
  useCreateSpecDecision,
  useUpdateSpecDocument,
} from "@multica/core/spec-memory";
import { useWorkspaceId } from "@multica/core/hooks";
import type { SpecDocument, SpecModule } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { toast } from "sonner";

const DEFAULT_DOC_KIND = "index";

export function ModuleDocumentsPanel({
  module,
  className,
}: {
  module: SpecModule | null;
  className?: string;
}) {
  const wsId = useWorkspaceId();
  const documents = useQuery(
    specDocumentListOptions(wsId, module?.id ?? ""),
  );
  const [selectedKind, setSelectedKind] = useState(DEFAULT_DOC_KIND);
  const [documentDirty, setDocumentDirty] = useState(false);

  const selectedDocument = useMemo(
    () =>
      documents.data?.find((document) => document.doc_kind === selectedKind) ??
      documents.data?.[0] ??
      null,
    [documents.data, selectedKind],
  );

  useEffect(() => {
    if (selectedDocument) setSelectedKind(selectedDocument.doc_kind);
  }, [selectedDocument]);

  const requestDocument = (docKind: string) => {
    if (
      documentDirty &&
      !window.confirm("Discard unsaved spec document changes?")
    ) {
      return;
    }
    setSelectedKind(docKind);
  };

  if (!module) {
    return (
      <section className={cn("flex min-h-72 items-center justify-center rounded-lg border border-dashed", className)}>
        <p className="text-sm text-muted-foreground">Select a module to edit durable spec docs.</p>
      </section>
    );
  }

  if (documents.isLoading) {
    return (
      <section className={cn("space-y-3", className)}>
        <Skeleton className="h-8 w-52" />
        <Skeleton className="h-96 w-full" />
      </section>
    );
  }

  return (
    <section className={cn("grid min-h-0 grid-cols-[220px_minmax(0,1fr)] gap-4", className)}>
      <aside className="min-h-0 rounded-lg border bg-background">
        <div className="border-b p-3">
          <div className="flex items-center gap-2 text-sm font-medium">
            <FileText className="size-4" />
            {module.key}
          </div>
          <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">
            {module.title}
          </p>
        </div>
        <div className="p-2">
          {(documents.data ?? []).map((document) => (
            <button
              key={document.id}
              type="button"
              onClick={() => requestDocument(document.doc_kind)}
              className={cn(
                "flex w-full items-center justify-between gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-muted",
                document.doc_kind === selectedKind && "bg-muted",
              )}
            >
              <span className="truncate">{document.title || document.doc_kind}</span>
              <Badge variant="outline" className="max-w-24 truncate">
                {document.doc_kind}
              </Badge>
            </button>
          ))}
          {(documents.data ?? []).length === 0 ? (
            <button
              type="button"
              onClick={() => requestDocument(DEFAULT_DOC_KIND)}
              className="flex w-full items-center justify-between gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-muted"
            >
              <span>00-index.md</span>
              <Badge variant="outline">{DEFAULT_DOC_KIND}</Badge>
            </button>
          ) : null}
        </div>
      </aside>
      <DocumentEditor
        moduleId={module.id}
        docKind={selectedDocument?.doc_kind ?? selectedKind}
        document={selectedDocument}
        onDirtyChange={setDocumentDirty}
      />
    </section>
  );
}

function DocumentEditor({
  moduleId,
  docKind,
  document,
  onDirtyChange,
}: {
  moduleId: string;
  docKind: string;
  document: SpecDocument | null;
  onDirtyChange: (dirty: boolean) => void;
}) {
  const updateDocument = useUpdateSpecDocument(moduleId, docKind);
  const createDecision = useCreateSpecDecision();
  const [title, setTitle] = useState("");
  const [sourcePath, setSourcePath] = useState("");
  const [body, setBody] = useState("");
  const [decisionTitle, setDecisionTitle] = useState("");
  const [decisionBody, setDecisionBody] = useState("");

  useEffect(() => {
    setTitle(document?.title ?? docKind);
    setSourcePath(document?.source_path ?? "");
    setBody(document?.body ?? "");
    onDirtyChange(false);
  }, [docKind, document]);

  useEffect(() => {
    onDirtyChange(
      title !== (document?.title ?? docKind) ||
        sourcePath !== (document?.source_path ?? "") ||
        body !== (document?.body ?? ""),
    );
  }, [body, docKind, document, onDirtyChange, sourcePath, title]);

  const save = () => {
    updateDocument.mutate(
      {
        title,
        source_path: sourcePath,
        body,
      },
      {
        onSuccess: () => {
          onDirtyChange(false);
          toast.success("Spec document saved");
        },
        onError: (error) =>
          toast.error(error instanceof Error ? error.message : "Save failed"),
      },
    );
  };

  const appendDecision = () => {
    createDecision.mutate(
      {
        module_id: moduleId,
        title: decisionTitle,
        body: decisionBody,
        source_path: document?.source_path ?? "",
      },
      {
        onSuccess: () => {
          setDecisionTitle("");
          setDecisionBody("");
          toast.success("Decision recorded");
        },
        onError: (error) =>
          toast.error(error instanceof Error ? error.message : "Save failed"),
      },
    );
  };

  return (
    <div className="grid min-h-0 gap-4">
      <div className="flex min-h-0 flex-col rounded-lg border bg-background">
      <div className="flex items-start justify-between gap-3 border-b p-3">
        <div className="grid min-w-0 flex-1 gap-2">
          <div className="grid gap-1.5">
            <Label className="text-xs">Title</Label>
            <Input value={title} onChange={(event) => setTitle(event.target.value)} />
          </div>
          <div className="grid gap-1.5">
            <Label className="text-xs">Source path</Label>
            <Input
              value={sourcePath}
              onChange={(event) => setSourcePath(event.target.value)}
              placeholder=".spec/epics/<epic>/modules/<module>/00-index.md"
            />
          </div>
        </div>
        <Button
          type="button"
          size="sm"
          onClick={save}
          disabled={updateDocument.isPending}
        >
          <Save className="size-3.5" />
          Save
        </Button>
      </div>
      <Textarea
        value={body}
        onChange={(event) => setBody(event.target.value)}
        className="min-h-[32rem] flex-1 resize-none rounded-none border-0 font-mono text-sm focus-visible:ring-0"
      />
      </div>
      <div className="rounded-lg border bg-background p-3">
        <div className="mb-3 text-sm font-medium">Append decision</div>
        <div className="grid gap-3">
          <div className="grid gap-1.5">
            <Label className="text-xs">Title</Label>
            <Input
              value={decisionTitle}
              onChange={(event) => setDecisionTitle(event.target.value)}
              placeholder="Architecture or product decision"
            />
          </div>
          <div className="grid gap-1.5">
            <Label className="text-xs">Body</Label>
            <Textarea
              value={decisionBody}
              onChange={(event) => setDecisionBody(event.target.value)}
              className="min-h-24 resize-y text-sm"
            />
          </div>
          <Button
            type="button"
            size="sm"
            onClick={appendDecision}
            disabled={createDecision.isPending || !decisionTitle.trim()}
          >
            <Save className="size-3.5" />
            Append decision
          </Button>
        </div>
      </div>
    </div>
  );
}

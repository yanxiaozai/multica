"use client";

import { useMemo } from "react";
import type { SpecEpic, SpecModule } from "@multica/core/types";

export function useSelectedSpecModule(
  epics: SpecEpic[] | undefined,
  modules: SpecModule[] | undefined,
  selectedEpicId: string,
  selectedModuleId: string,
) {
  return useMemo(
    () => ({
      epic: epics?.find((epic) => epic.id === selectedEpicId) ?? null,
      module:
        modules?.find((module) => module.id === selectedModuleId) ?? null,
    }),
    [epics, modules, selectedEpicId, selectedModuleId],
  );
}

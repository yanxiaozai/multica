"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../../platform/storage";

export type TranscriptSortDirection = "chronological" | "newest_first";
export type TranscriptViewMode = "events" | "raw";

interface TranscriptViewState {
  sortDirection: TranscriptSortDirection;
  viewMode: TranscriptViewMode;
  setSortDirection: (dir: TranscriptSortDirection) => void;
  setViewMode: (mode: TranscriptViewMode) => void;
}

export const useTranscriptViewStore = create<TranscriptViewState>()(
  persist(
    (set) => ({
      sortDirection: "chronological",
      viewMode: "events",
      setSortDirection: (sortDirection) => set({ sortDirection }),
      setViewMode: (viewMode) => set({ viewMode }),
    }),
    {
      name: "multica_transcript_view",
      storage: createJSONStorage(() => defaultStorage),
      partialize: (state) => ({
        sortDirection: state.sortDirection,
        viewMode: state.viewMode,
      }),
    },
  ),
);

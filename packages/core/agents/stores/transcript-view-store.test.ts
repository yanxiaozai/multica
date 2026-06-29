import { beforeEach, describe, expect, it } from "vitest";
import { useTranscriptViewStore } from "./transcript-view-store";

beforeEach(() => {
  useTranscriptViewStore.setState({ sortDirection: "chronological", viewMode: "events" });
});

describe("useTranscriptViewStore", () => {
  it("defaults to chronological events mode so existing readers see no behavior change", () => {
    expect(useTranscriptViewStore.getState().sortDirection).toBe("chronological");
    expect(useTranscriptViewStore.getState().viewMode).toBe("events");
  });

  it("setSortDirection switches between the two known directions", () => {
    const { setSortDirection } = useTranscriptViewStore.getState();

    setSortDirection("newest_first");
    expect(useTranscriptViewStore.getState().sortDirection).toBe("newest_first");

    setSortDirection("chronological");
    expect(useTranscriptViewStore.getState().sortDirection).toBe("chronological");
  });

  it("setViewMode switches between event and raw transcript views", () => {
    const { setViewMode } = useTranscriptViewStore.getState();

    setViewMode("raw");
    expect(useTranscriptViewStore.getState().viewMode).toBe("raw");

    setViewMode("events");
    expect(useTranscriptViewStore.getState().viewMode).toBe("events");
  });
});

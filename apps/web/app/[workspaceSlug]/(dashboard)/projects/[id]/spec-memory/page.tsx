"use client";

import { use } from "react";
import { SpecMemoryPage } from "@multica/views/spec-memory";

export default function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <SpecMemoryPage projectId={id} />;
}

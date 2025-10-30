import type { LiveTemplateClient } from "../livetemplate-client";
import type { UpdateResult } from "../types";

/**
 * Utility function to load and apply updates from JSON files.
 */
export async function loadAndApplyUpdate(
  client: LiveTemplateClient,
  updatePath: string
): Promise<UpdateResult> {
  try {
    const nodeRequire = (globalThis as any)?.require;
    if (typeof nodeRequire === "function") {
      const fs = nodeRequire("fs");
      const updateData = JSON.parse(fs.readFileSync(updatePath, "utf8"));
      return client.applyUpdate(updateData);
    }

    const response = await fetch(updatePath);
    const updateData = await response.json();
    return client.applyUpdate(updateData);
  } catch (error) {
    throw new Error(`Failed to load update from ${updatePath}: ${error}`);
  }
}

/**
 * Compare two HTML strings, ignoring whitespace differences.
 */
export function compareHTML(
  expected: string,
  actual: string
): {
  match: boolean;
  differences: string[];
} {
  const differences: string[] = [];

  const normalizeHTML = (html: string) => {
    return html.replace(/\s+/g, " ").replace(/>\s+</g, "><").trim();
  };

  const normalizedExpected = normalizeHTML(expected);
  const normalizedActual = normalizeHTML(actual);

  if (normalizedExpected === normalizedActual) {
    return { match: true, differences: [] };
  }

  const expectedLines = normalizedExpected.split("\n");
  const actualLines = normalizedActual.split("\n");
  const maxLines = Math.max(expectedLines.length, actualLines.length);

  for (let i = 0; i < maxLines; i++) {
    const expectedLine = expectedLines[i] || "";
    const actualLine = actualLines[i] || "";

    if (expectedLine !== actualLine) {
      differences.push(`Line ${i + 1}:`);
      differences.push(`  Expected: ${expectedLine}`);
      differences.push(`  Actual:   ${actualLine}`);
    }
  }

  return { match: false, differences };
}

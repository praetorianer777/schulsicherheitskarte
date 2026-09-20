import axeCore from "axe-core";

/**
 * Runs axe over the rendered container and returns the violations.
 *
 * These checks catch regressions — a missing label, a heading level skipped, a
 * control that cannot be reached. They do not replace testing with a keyboard
 * and a screen reader, and nothing here should be read as saying they do.
 */
export async function axe(container: Element) {
  const results = await axeCore.run(container, {
    rules: {
      // The container is a fragment of a page, so page-level landmark and
      // region rules would fire on every component test.
      region: { enabled: false },
    },
  });
  return results.violations;
}

export function describeViolations(violations: Awaited<ReturnType<typeof axe>>) {
  return violations
    .map((v) => `${v.id}: ${v.help}\n  ${v.nodes.map((n) => n.html).join("\n  ")}`)
    .join("\n");
}

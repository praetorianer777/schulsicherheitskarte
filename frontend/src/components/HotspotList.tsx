import type { Hotspot } from "../api/types";
import { formatNumber } from "../lib/format";

type Props = {
  hotspots: Hotspot[];
  onSelect: (hotspot: Hotspot) => void;
};

/**
 * The ranked equivalent of the circles on the map. Selecting an entry moves the
 * map, so the list is a way of using the map rather than a consolation prize
 * for not being able to.
 */
export default function HotspotList({ hotspots, onSelect }: Props) {
  return (
    <section aria-labelledby="schwerpunkte">
      <h2 id="schwerpunkte" className="text-lg font-semibold">
        Unfallschwerpunkte
      </h2>

      {hotspots.length === 0 ? (
        <p className="mt-2 text-ink-muted">
          Im gewählten Umkreis gibt es keine Stelle mit mindestens zwei Unfällen innerhalb von
          50 Metern. Einzelne Unfälle werden hier bewusst nicht als Schwerpunkt geführt.
        </p>
      ) : (
        <ol className="mt-2 divide-y divide-line rounded border border-line bg-white">
          {hotspots.map((hotspot, index) => (
            <li key={hotspot.id}>
              <button
                type="button"
                onClick={() => onSelect(hotspot)}
                className="flex w-full items-baseline gap-3 px-4 py-3 text-left hover:bg-surface"
              >
                <span className="text-lg font-semibold tabular-nums">{index + 1}.</span>
                <span className="flex-1">
                  <span className="font-medium">
                    {formatNumber(hotspot.accidentCount)} Unfälle, {formatNumber(hotspot.distance)} m
                    entfernt
                  </span>
                  <span className="block text-sm text-ink-muted">
                    {describeBreakdown(hotspot)} · {hotspot.firstYear}–{hotspot.lastYear} ·
                    Gefahrenindex {hotspot.score.toFixed(1)}
                  </span>
                </span>
                <span className="text-sm text-accent">Auf der Karte zeigen</span>
              </button>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

function describeBreakdown(hotspot: Hotspot): string {
  const parts: string[] = [];
  const add = (count: number | undefined, singular: string, plural: string) => {
    if (count && count > 0) parts.push(`${count} ${count === 1 ? singular : plural}`);
  };
  add(hotspot.breakdown.fatal, "Getöteter", "Getötete");
  add(hotspot.breakdown.serious, "Schwerverletzter", "Schwerverletzte");
  add(hotspot.breakdown.slight, "Leichtverletzter", "Leichtverletzte");
  add(hotspot.breakdown.pedestrian, "zu Fuß", "zu Fuß");
  add(hotspot.breakdown.bike, "mit Rad", "mit Rad");
  return parts.join(", ");
}

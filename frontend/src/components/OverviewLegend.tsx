import { useId } from "react";

import { formatNumber } from "../lib/format";
import { mapColours, nearbyRadiusMetres, nearbySteps, type NearbyStep } from "../lib/palette";

type Props = {
  scaled: boolean;
  onScaledChange: (scaled: boolean) => void;
};

function stepLabel(step: NearbyStep): string {
  if (step.to === null) return `${formatNumber(step.from)} und mehr`;
  if (step.from === step.to) return step.from === 0 ? "keiner" : formatNumber(step.from);
  return `${formatNumber(step.from)}–${formatNumber(step.to)}`;
}

/**
 * What the points on the overview stand for — and what they do not. Without
 * the second part the map ranks schools by how much traffic passes them.
 */
export default function OverviewLegend({ scaled, onScaledChange }: Props) {
  const toggleId = useId();

  return (
    <section aria-labelledby="uebersicht-legende" className="rounded border border-line bg-white p-4">
      <h2 id="uebersicht-legende" className="text-sm font-semibold">
        Zeichenerklärung
      </h2>

      <div className="mt-2 flex items-center gap-2">
        <input
          id={toggleId}
          type="checkbox"
          checked={scaled}
          onChange={(event) => onScaledChange(event.target.checked)}
        />
        <label htmlFor={toggleId} className="text-sm">
          Punkte nach Unfällen im Umkreis zeigen
        </label>
      </div>

      {scaled ? (
        <>
          <p className="mt-3 text-sm">
            Unfälle mit Personenschaden im Umkreis von {formatNumber(nearbyRadiusMetres)} m, alle
            importierten Berichtsjahre:
          </p>
          <ul className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
            {nearbySteps.map((step) => (
              <li key={step.from} className="flex items-center gap-2">
                <svg width="28" height="28" aria-hidden="true" className="shrink-0">
                  <circle
                    cx="14"
                    cy="14"
                    r={step.radius}
                    fill={step.colour}
                    stroke={mapColours.ring}
                    strokeWidth="2"
                  />
                </svg>
                {stepLabel(step)}
              </li>
            ))}
          </ul>
          <p className="mt-3 text-sm text-ink-muted">
            Das sind absolute Zahlen ohne Verkehrsmengen. An einer Schule in der Innenstadt
            fährt mehr Verkehr vorbei als an einer im Dorf — mehr Unfälle im Umkreis heißt nicht,
            dass ihr Schulweg gefährlicher ist. Schwarze Punkte sind noch nicht ausgezählt.
          </p>
        </>
      ) : (
        <p className="mt-3 text-sm text-ink-muted">
          Jeder Punkt ist eine Schule oder Kita; Zahlen zeigen gruppierte Einrichtungen, keine
          Unfälle.
        </p>
      )}
    </section>
  );
}

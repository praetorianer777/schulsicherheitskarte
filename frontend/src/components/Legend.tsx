import { mapColours, severityRadius } from "../lib/palette";
import { severityLabel } from "../lib/format";

/**
 * Severity is drawn as size, not as colour, so the legend has to show the
 * sizes. It also states each colour in words — no mark on this map means
 * anything by its colour alone.
 */
export default function Legend() {
  return (
    <section aria-labelledby="zeichenerklaerung" className="rounded border border-line bg-white p-4">
      <h2 id="zeichenerklaerung" className="text-sm font-semibold">
        Zeichenerklärung
      </h2>

      <dl className="mt-3 space-y-2 text-sm">
        <div>
          <dt className="font-medium">Unfälle mit Personenschaden</dt>
          <dd className="mt-1 flex flex-wrap items-center gap-4">
            {([1, 2, 3] as const).map((severity) => (
              <span key={severity} className="flex items-center gap-2">
                <svg width="20" height="20" aria-hidden="true" className="shrink-0">
                  <circle
                    cx="10"
                    cy="10"
                    r={severityRadius[severity]}
                    fill={mapColours.accident}
                    stroke={mapColours.ring}
                    strokeWidth="1.5"
                  />
                </svg>
                {severityLabel[severity]}
              </span>
            ))}
          </dd>
        </div>

        <div>
          <dt className="font-medium">Unfallschwerpunkte</dt>
          <dd className="text-ink-muted">
            Rote Kreise, nummeriert nach Rang. Die Fläche entspricht dem Gefahrenindex.
          </dd>
        </div>

        <div>
          <dt className="font-medium">Gemeldete Gefahrenstellen</dt>
          <dd className="mt-1">
            <Swatch colour={mapColours.report} label="von Eltern gemeldet und freigegeben" />
          </dd>
        </div>

        <div>
          <dt className="font-medium">Infrastruktur</dt>
          <dd className="mt-1 flex flex-wrap items-center gap-4">
            <Swatch colour={mapColours.crossing} label="Querungshilfe" />
            <Swatch colour={mapColours.trafficSignals} label="Ampel" />
            <Swatch colour={mapColours.trafficCalming} label="Verkehrsberuhigung" />
            <span className="flex items-center gap-2">
              <svg width="20" height="20" aria-hidden="true" className="shrink-0">
                <line x1="2" y1="10" x2="18" y2="10" stroke={mapColours.speedLimit} strokeWidth="3" />
              </svg>
              Tempolimit
            </span>
          </dd>
        </div>
      </dl>
    </section>
  );
}

function Swatch({ colour, label }: { colour: string; label: string }) {
  return (
    <span className="flex items-center gap-2">
      <svg width="20" height="20" aria-hidden="true" className="shrink-0">
        <circle cx="10" cy="10" r="5" fill={colour} stroke={mapColours.ring} strokeWidth="1.5" />
      </svg>
      {label}
    </span>
  );
}

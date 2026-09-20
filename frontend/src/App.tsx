import { Link, Route, Routes } from "react-router-dom";

import FactsheetPage from "./pages/FactsheetPage";
import InstitutionPage from "./pages/InstitutionPage";
import ModerationPage from "./pages/ModerationPage";
import StartPage from "./pages/StartPage";

export default function App() {
  return (
    <div className="min-h-screen flex flex-col">
      <a className="skip-link" href="#inhalt">
        Zum Inhalt springen
      </a>

      <header className="border-b border-line bg-white">
        <div className="mx-auto max-w-6xl px-4 py-3">
          <Link to="/" className="text-lg font-semibold no-underline">
            <span aria-hidden="true">🚸 </span>Schulweg-Sicherheitskarte
          </Link>
        </div>
      </header>

      <main id="inhalt" className="flex-1">
        <Routes>
          <Route path="/" element={<StartPage />} />
          <Route path="/einrichtung/:id" element={<InstitutionPage />} />
          <Route path="/moderation" element={<ModerationPage />} />
          <Route path="/einrichtung/:id/faktenblatt" element={<FactsheetPage />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>

      <footer className="site border-t border-line bg-white">
        <div className="mx-auto max-w-6xl px-4 py-4 text-sm text-ink-muted">
          <p>
            Unfalldaten:{" "}
            <a href="https://unfallatlas.statistikportal.de/">Unfallatlas der Statistischen Ämter</a>{" "}
            (dl-de/by-2-0). Karte und Einrichtungen:{" "}
            <a href="https://www.openstreetmap.org/copyright">© OpenStreetMap-Mitwirkende</a> (ODbL).
            Kartengrundlage: © basemap.de / BKG.
          </p>
        </div>
      </footer>
    </div>
  );
}

function NotFound() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-12">
      <h1 className="text-2xl font-semibold">Seite nicht gefunden</h1>
      <p className="mt-2">
        <Link to="/">Zurück zur Suche</Link>
      </p>
    </div>
  );
}

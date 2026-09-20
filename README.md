# 🚸 Schulweg-Sicherheitskarte

Unfallschwerpunkte rund um jede Schule und jede Kita, gebaut aus offenen Daten — damit
Elternvertretungen für einen Zebrastreifen oder Tempo 30 mit Zahlen argumentieren
können statt mit Anekdoten.

> Diese README und die Weboberfläche sind deutsch, weil die Menschen deutsch sind, für
> die dieses Projekt gebaut wird. Alles andere im Repository — Quellcode, Kommentare,
> Commits, Issues, Pull Requests — ist englisch.

## Worum es geht

Seit der StVO-Novelle 2024 können Kommunen Tempo 30 vor Schulen und Kitas deutlich
leichter anordnen. Was im Einzelfall meist fehlt, sind belastbare Belege. Die gibt es,
aber verstreut:

- Der **Unfallatlas** der Statistischen Ämter veröffentlicht jeden Straßenverkehrsunfall
  mit Personenschaden als offenen, georeferenzierten Datensatz — inklusive der Angabe,
  ob ein Fußgänger oder eine Radfahrerin beteiligt war.
- **OpenStreetMap** weiß, wo Schulen, Kitas, Querungshilfen, Ampeln und Tempolimits sind.

Zusammengeführt hat das bisher niemand pro Schule. Genau das macht dieses Projekt,
ergänzt es um eine Meldefunktion für Beinahe-Unfälle, die in keiner Statistik auftauchen,
und erzeugt daraus ein druckbares Faktenblatt für die nächste Verkehrsschau.

## Datenquellen

| Quelle | Inhalt | Lizenz |
|---|---|---|
| [Unfallatlas](https://unfallatlas.statistikportal.de/), Berichtsjahre 2016–2025 | Unfälle mit Personenschaden, Punktgeometrie, Beteiligung von Fuß- und Radverkehr | `dl-de/by-2-0` |
| [OpenStreetMap](https://www.openstreetmap.org/) über die Overpass-API | Schulen, Kitas, Querungshilfen, Ampeln, verkehrsberuhigende Maßnahmen, Tempolimits | ODbL, Namensnennung „© OpenStreetMap-Mitwirkende“ |

### Was die Daten nicht hergeben

Dieser Abschnitt steht auf jedem Faktenblatt, denn eine Behauptung, die über die Daten
hinausgeht, wird in der ersten Verkehrsschau auseinandergenommen:

- Erfasst sind ausschließlich Unfälle **mit Personenschaden**. Beinahe-Unfälle,
  Sachschäden und alltägliche Bedrängung sind unsichtbar — dafür gibt es die
  Meldefunktion.
- Die Koordinaten sind auf das Straßennetz gerastert und anonymisiert. Ein Punkt
  bezeichnet einen Straßenabschnitt, nicht eine Stelle auf dem Asphalt.
- Es gibt keine Angaben zur Verkehrsmenge. Damit sind das **absolute Häufigkeiten, keine
  Risikoraten**: Eine ruhige Straße mit einem Unfall ist nicht automatisch sicherer als
  eine stark befahrene mit dreien.
- Dass an einer Stelle nichts passiert ist, belegt nicht, dass sie sicher ist.

## Barrierefreiheit

Keine Verhandlungsmasse und kein späterer Aufräumdurchgang. Ziel ist **WCAG 2.2, Stufe AA**:

- Zu jeder Karte gibt es eine gleichwertige Listenansicht — Karteninhalte sind nie der
  einzige Weg zur Information.
- Vollständig mit der Tastatur bedienbar, sichtbarer Fokus, keine Tastaturfallen.
- Die Unfallschwere wird nie allein über Farbe ausgedrückt.
- Korrekte Dokumentsprache, saubere Überschriftenstruktur, beschriftete Formularfelder,
  Live-Regionen für asynchron nachgeladene Ergebnisse.
- `prefers-reduced-motion` wird für Kartenanimationen respektiert.
- Automatisierte `axe-core`-Prüfungen laufen in der Testsuite mit. Sie fangen Rückschritte
  ab, ersetzen aber keinen manuellen Test mit Tastatur und Screenreader.

## Technik

- **Backend** — Go, PostgreSQL + PostGIS
- **Frontend** — React, TypeScript, TailwindCSS, MapLibre GL
- **Betrieb** — Docker Compose, selbst gehostet

## Entwicklung

```bash
./run-tests.sh          # der eine Einstiegspunkt: Shell, Go, Frontend, End-to-End
```

Jede Änderung beginnt bei einem GitHub-Issue und lebt auf einem Branch
`<type>/<issue>-<slug>`. `.claude/hooks/branch-guard.sh` setzt das durch: Bearbeitungen,
Commits und Pushes außerhalb eines Issue-Branches werden abgelehnt, Pushes auf `main`
ebenfalls, und vor jedem Push läuft `./run-tests.sh`.

## Lizenz

[AGPL-3.0](LICENSE). Dieses Projekt wird betrieben und nicht weitergegeben — genau
deshalb die Netzwerkklausel: Wer eine veränderte Fassung öffentlich betreibt, muss seine
Änderungen veröffentlichen.

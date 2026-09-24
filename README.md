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
| [OpenStreetMap](https://www.openstreetmap.org/) über die Overpass-API | Schulen, Kitas, Gemeinde- und Ortsteilgrenzen, Querungshilfen, Ampeln, verkehrsberuhigende Maßnahmen, Tempolimits | ODbL, Namensnennung „© OpenStreetMap-Mitwirkende“ |

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

## Methode

Ein **Unfallschwerpunkt** ist ein Kreis mit 50 m Radius, in dem mindestens zwei Unfälle
liegen. Das Verfahren nimmt wiederholt den Kreis mit den meisten noch nicht zugeordneten
Unfällen. Zwei Unfälle desselben Schwerpunkts liegen dadurch nie weiter als 100 m
auseinander — eine Stelle, auf die man bei einer Verkehrsschau zeigen kann.

Der naheliegende Algorithmus DBSCAN wurde zuerst probiert und verworfen: Er verkettet
A mit B und B mit C und zieht im Stadtzentrum ganze Straßenzüge zusammen. Im Landkreis
Zwickau entstand so ein „Schwerpunkt“ von 621 m Ausdehnung mit 72 Unfällen — und genau
diese Klumpen standen ganz oben in der Rangliste.

Jeder Schwerpunkt bekommt einen **Gefahrenindex**, die Summe über seine Unfälle:

```
Index = Σ  Gewicht_Schwere × Gewicht_Beteiligung × Gewicht_Aktualität
```

| Faktor | Wert | Begründung |
|---|---|---|
| Getötete | 10 | Ein Toter ist nicht fünf Leichtverletzte. Das Verhältnis ist eine Setzung — sie steht hier, statt in einer Abfrage versteckt zu sein. |
| Schwerverletzte | 5 | |
| Leichtverletzte | 1 | |
| Fuß- oder Radbeteiligung | × 3 | Es geht um den Schulweg. Ein Auffahrunfall zwischen zwei Autos sagt darüber weniger aus als ein angefahrenes Kind an derselben Stelle. |
| Aktualität | Halbwertszeit 4 Jahre | Infrastruktur ändert sich. Eine 2018 gebaute Querungshilfe wird nicht durch das beantwortet, was 2016 geschah. Vier Jahre sind kurz genug, um einer solchen Änderung zu folgen, und lang genug, dass ein ruhiges Jahr eine bekannte Gefahrenstelle nicht auslöscht. |

Die Aktualität zählt vom **letzten importierten Berichtsjahr** zurück, nicht vom heutigen
Datum. Damit hängt der Index von den Daten ab und nicht davon, wann jemand die Seite
öffnet: Zwei Menschen, die dasselbe Faktenblatt einen Monat auseinander lesen, sehen
dieselbe Zahl.

Ein einzelner Unfall ist **kein** Schwerpunkt. Ihn als Muster zu präsentieren ist das,
woran ein Faktenblatt in der ersten Verkehrsschau scheitert.

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

## Oberfläche

Die Startseite zeigt alle Schulen und Kitas auf einer Karte und sucht sie nach Namen.
Jeder Punkt steht für die Unfälle mit Personenschaden im Umkreis von 500 m über alle
importierten Jahre — in fünf Stufen, größer und dunkler, in einem einzigen Violett. Das
ist keine Ampel: Ohne Verkehrsmengen sagt die Zahl nicht, welcher Schulweg gefährlicher
ist, und die Zeichenerklärung sagt das auch. Dieselbe Zahl steht in der Trefferliste und
beim Überfahren eines Punkts; ein Schalter zeigt alle Punkte wieder gleich.

Die Einrichtungsseite zeigt die
Unfälle im Umkreis auf einer Karte, dazu Kennzahlen, die nach Gefahrenindex sortierten
Schwerpunkte, die Querungshilfen, Ampeln und Tempolimits ringsum — und jeden einzelnen
Unfall als Tabellenzeile. Umkreis (250/500/1000 m), Zeitraum und der Schulweg-Blick
(„nur Fuß- und Radbeteiligung") lassen sich umstellen.

Die Unfallschwere wird auf der Karte über die **Größe** der Punkte gezeigt, nicht über
Farbe: Drei Rottöne wären auf einer Karte weder sicher unterscheidbar noch
kontraststark genug. Farbe trennt die Objektarten, und keine davon bedeutet etwas
allein durch ihre Farbe — die Zeichenerklärung benennt jede, und die Tabelle enthält
alles noch einmal in Worten.

## Gemeldete Gefahrenstellen

Beinahe-Unfälle tauchen in keiner Unfallstatistik auf. Eltern können solche Stellen
deshalb selbst melden — mit Kategorie, kurzer Beschreibung und einem Punkt auf der Karte,
der sich auch ohne Maus in 25-Meter-Schritten setzen lässt.

Eine Meldung ist **nicht sofort öffentlich**. Sie wird gesichtet und erscheint erst nach
Freigabe. Gespeichert wird weder Name noch Adresse, sondern nur ein gesalzener Prüfwert
der Absenderadresse — genug, um „derselbe Absender nochmal" zu erkennen, und wertlos,
sobald das Salt gewechselt wird.

## Faktenblatt

Jede Einrichtungsseite führt zu einem druckbaren Faktenblatt für die Verkehrsschau: eine
A4-Seite mit Anschrift an die Straßenverkehrsbehörde, Kennzahlen, den Schwerpunkten als
nummerierte Tabelle samt passender Karte, der vorhandenen Infrastruktur — und zwei
Abschnitten, die nicht weggelassen werden: **wie der Gefahrenindex zustande kommt** und
**was die Zahlen nicht hergeben**.

Umkreis und Zeitraum stehen in der Adresse der Seite. Derselbe Link ergibt dieselben
Zahlen — wer das Blatt weitergibt, gibt nicht versehentlich andere Zahlen weiter.

## Technik

- **Backend** — Go, PostgreSQL + PostGIS
- **Frontend** — React, TypeScript, TailwindCSS, MapLibre GL
- **Betrieb** — Docker Compose, selbst gehostet

## Daten importieren

```bash
docker compose -f deploy/docker-compose.yml up -d
docker compose -f deploy/docker-compose.yml run --rm importer accidents -years 2016-2025
docker compose -f deploy/docker-compose.yml run --rm importer osm
```

Welches Gebiet importiert wird, steht in [`regions.yaml`](regions.yaml). Derzeit ist das
der **Landkreis Zwickau** (AGS `14524`), der St. Egidien und das Gebiet des früheren
Landkreises Zwickauer Land umfasst.

Der Importer ist auf Wiederholung ausgelegt: Heruntergeladene Archive werden per ETag
revalidiert statt erneut geladen, und ein erneuter Import derselben Datei schreibt keine
Zeile. Jeder Lauf landet in `import_runs` — auch ein gescheiterter, damit sich jede Zahl
auf der Karte auf den Import zurückführen lässt, aus dem sie stammt.

Beim OSM-Import werden die Rohantworten der Overpass-API mitgeschrieben.
`importer osm -offline` spielt sie erneut ein, statt den ehrenamtlich betriebenen Dienst
noch einmal zu belasten — nützlich beim Entwickeln und beim Nachvollziehen alter Stände.

Einrichtungen, die in OpenStreetMap doppelt erfasst sind — einmal als Gelände, einmal als
Punkt darin —, werden beim Import zu einer zusammengeführt. Sonst stünden zwei Marker auf
der Karte und die Unfälle rund um eine Schule verteilten sich auf zwei Seiten. Objekte,
die in OpenStreetMap verschwinden, werden beim nächsten Import aus dem importierten
Gebiet entfernt.

## Selbst betreiben

[DEPLOY.md](DEPLOY.md) beschreibt eine Testinstallation mit Docker Compose — vom leeren
Rechner bis zu einer API, die echte Unfalldaten für das konfigurierte Gebiet beantwortet.

## Entwicklung

```bash
./run-tests.sh          # der eine Einstiegspunkt: Shell, Go, Frontend, End-to-End
```

Die End-to-End-Stufe fährt den Compose-Stack hoch, spielt feste Testdaten ein, berechnet
die Schwerpunkte mit dem echten Importer und bedient die Seite mit einem Browser. Sie
räumt danach auf, auch wenn sie fehlschlägt.

Jede Änderung beginnt bei einem GitHub-Issue und lebt auf einem Branch
`<type>/<issue>-<slug>`. `.claude/hooks/branch-guard.sh` setzt das durch: Bearbeitungen,
Commits und Pushes außerhalb eines Issue-Branches werden abgelehnt, Pushes auf `main`
ebenfalls, und vor jedem Push läuft `./run-tests.sh`.

## API

```
GET /api/institutions?q=goethe                    Suche nach Name
GET /api/institutions?bbox=minLon,minLat,maxLon,maxLat
GET /api/institutions/{id}
GET /api/institutions/{id}/accidents              ?radius=500&from=2016&to=2025&modes=foot,bike
GET /api/institutions/{id}/hotspots               ?radius=500
GET /api/institutions/{id}/infrastructure         ?radius=500
GET /api/institutions/{id}/factsheet              ?radius=500&from=&to=&modes=
GET /healthz
```

Jede Antwort, die Daten enthält, trägt ihre Quellenangaben mit — beide Lizenzen
verlangen Namensnennung, und eine Karte, die sie nur in einer Fußzeile führt, verliert
sie in dem Moment, in dem jemand die API direkt nutzt.

`modes=foot,bike` bedeutet „zu Fuß **oder** mit dem Rad beteiligt", nicht beides
gleichzeitig. Ein fehlerhafter Parameter beantwortet sich selbst: Die Antwort nennt den
Parameter und den Grund, statt einem 400 ohne Erklärung oder einem 500.

## Lizenz

[AGPL-3.0](LICENSE). Dieses Projekt wird betrieben und nicht weitergegeben — genau
deshalb die Netzwerkklausel: Wer eine veränderte Fassung öffentlich betreibt, muss seine
Änderungen veröffentlichen.

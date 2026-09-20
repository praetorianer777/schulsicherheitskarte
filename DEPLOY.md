# Testinstallation

Anleitung für einen **Probebetrieb** auf einem eigenen Rechner oder Server.

> **Was hier noch nicht entsteht:** keine Verschlüsselung und kein Impressum. Diese Installation gehört noch
> **nicht ins offene Internet** — sondern ins lokale Netz oder hinter einen Reverse Proxy,
> der die Verschlüsselung übernimmt.

## Voraussetzungen

- Docker mit Compose v2 (`docker compose version`)
- **kein** Go, kein Node: die Images werden fertig aus der GitHub-Registry geladen
- etwa **4 GB freier Plattenplatz**: rund 1 GB für die Images, 300 MB für die
  heruntergeladenen Unfallatlas-Archive, der Rest für die Datenbank
- eine Internetverbindung für den Import — danach läuft alles offline
- Es werden keine Zugangsdaten und keine API-Schlüssel benötigt. Beide Datenquellen sind
  offen.

## 1. Holen und konfigurieren

```bash
git clone https://github.com/praetorianer777/schulsicherheitskarte.git
cd schulsicherheitskarte
cp deploy/.env.example deploy/.env
```

In `deploy/.env` mindestens das Datenbankkennwort ändern. Die Datenbank wird nicht auf
dem Host veröffentlicht, aber ein Standardkennwort bleibt ein Standardkennwort.

Welches Gebiet importiert wird, steht in [`regions.yaml`](regions.yaml). Voreingestellt
ist der **Landkreis Zwickau**. Für ein anderes Gebiet dort den amtlichen
Gemeindeschlüssel und die Bounding-Box eintragen — beide müssen dasselbe Gebiet meinen.

## 2. Starten

```bash
docker compose -f deploy/docker-compose.yml pull
docker compose -f deploy/docker-compose.yml up -d
```

Die Images kommen aus `ghcr.io/praetorianer777/schulsicherheitskarte/*` und werden bei
jedem Stand von `main` neu gebaut. Sie werden nicht auf dieser Maschine gebaut — dafür
wäre eine Go-Werkzeugkette nötig, und zwei am selben Tag aktualisierte Maschinen liefen
sonst nicht zwangsläufig mit denselben Bytes.

> **Pakete auf öffentlich stellen.** Pakete in der GitHub-Registry sind anfangs privat,
> auch bei einem öffentlichen Repository. Einmalig unter *Packages* → jeweiliges Paket →
> *Package settings* → *Change visibility* auf öffentlich setzen. Alternativ meldet sich
> die Maschine an:
> `echo <token> | docker login ghcr.io -u <benutzername> --password-stdin`
> (Token mit dem Recht `read:packages`).

Wer aus dem Quellcode bauen will statt zu laden, nimmt weiterhin
`docker compose -f deploy/docker-compose.yml up -d --build`.

Der Start bringt die Datenbank hoch, wendet die Migrationen an und startet die API. Wenn
alles läuft:

```bash
docker compose -f deploy/docker-compose.yml ps
```

Beide Dienste müssen `healthy` melden. Der Dienst `migrate` ist ein Job und erscheint als
beendet — das ist richtig so.

## 3. Daten importieren

Die Reihenfolge ist nicht beliebig: Schwerpunkte werden aus den Unfällen berechnet, also
müssen die zuerst da sein.

```bash
cd deploy

# Unfallatlas, zehn Berichtsjahre. Lädt rund 300 MB, dauert einige Minuten.
docker compose run --rm importer accidents -years 2016-2025

# Schulen, Kitas, Querungen, Ampeln, Tempolimits aus OpenStreetMap.
docker compose run --rm importer osm

# Schwerpunkte berechnen. Ohne diesen Schritt bleibt die Schwerpunktliste leer.
docker compose run --rm importer hotspots
```

Erwartete Ausgabe für den Landkreis Zwickau:

```
2016: 151673 rows read, 150756 outside the configured regions, 917 newly imported
…
2025: 273007 rows read, 272207 outside the configured regions, 800 newly imported
Landkreis Zwickau: 443 schools and kindergartens written, 0 gone from OpenStreetMap and removed
Landkreis Zwickau/crossings: 2201 written, 0 removed
Landkreis Zwickau/speed_limits: 15417 written, 0 removed
8332 accidents clustered into 1426 hotspots, counting back from reporting year 2025
```

Der OSM-Import fragt die **Overpass-API** ab, einen ehrenamtlich betriebenen Dienst.
Wenn er gerade ausgelastet ist, wartet der Importer und versucht es erneut. Bei
`overpass is busy` einfach später noch einmal laufen lassen — oder mit
`importer osm -offline` die zuletzt gespeicherte Antwort einspielen.

## 4. Prüfen, ob es funktioniert hat

```bash
curl -s localhost:8080/healthz
# {"status":"ok"}

curl -s 'localhost:8080/api/institutions?q=grundschule' | jq '.institutions[:3]'

# Unfälle im Umkreis von 500 m um eine Schule, nur Fuß- und Radbeteiligung
ID=$(curl -s 'localhost:8080/api/institutions?q=Peter%20Breuer' | jq -r '.institutions[0].id')
curl -s "localhost:8080/api/institutions/$ID/accidents?radius=500&modes=foot,bike" | jq '.summary'
```

Wenn `summary.total` größer als null ist und `sources` die beiden Lizenzen nennt, steht
die Installation.

Die Karte selbst liegt unter `http://localhost:8090` — Schule suchen, auf einen Treffer
klicken.

## 4a. Hinter einem Reverse Proxy

Läuft der Reverse Proxy — etwa Nginx Proxy Manager — auf einer **anderen Maschine**,
sind zwei Einstellungen wichtig.

**Ein Proxy Host, nicht zwei.** Der Web-Container liefert die Seite aus *und* reicht
`/api` intern an die API weiter. Seite und Daten teilen sich damit eine Herkunft, der
Proxy braucht nur einen Host, und CORS kommt nirgends vor:

```
WEB_BIND=192.168.1.20     # LAN-Adresse dieses Docker-Hosts
WEB_PORT=8090
API_BIND=127.0.0.1        # die API wird über die Seite erreicht, nicht direkt
```

Im Proxy Manager dann ein Proxy Host mit *Forward Hostname/IP* = `192.168.1.20` und
*Forward Port* = `8090`. Läuft der Proxy auf derselben Maschine, ist auch für `WEB_BIND`
`127.0.0.1` die richtige Antwort.

**Ob die API dem Proxy glaubt.** Hinter einem Proxy stammt jede Anfrage scheinbar vom
Proxy. Die echte Client-Adresse steht dann nur im Header `X-Forwarded-For`:

```
TRUST_PROXY_HEADERS=true
```

Dieser Schalter ist voreingestellt **aus**, und das muss er sein: Der Header ist das, was
der Aufrufer hineinschreibt. Solange der API-Port direkt erreichbar ist, kann jeder eine
beliebige Adresse behaupten. Ihn einzuschalten ist die Aussage „der Proxy ist der einzige
Weg herein" — und die stimmt nur zusammen mit einem passend gesetzten `API_BIND`. Die
spätere Begrenzung der Meldungen pro Absender zählt genau diese Adresse.

Die Datenbank wird nie auf dem Host veröffentlicht. Sie ist nur innerhalb des
Compose-Netzes erreichbar, und dabei sollte es bleiben.

## 4b. Meldefunktion und Moderation

Eltern können Gefahrenstellen melden. Eine Meldung ist **nicht sofort öffentlich**: Sie
landet in einer Warteschlange und erscheint erst nach Freigabe auf der Karte.

Dafür sind zwei Werte nötig:

```
MODERATION_TOKEN=<lange Zufallszeichenkette>   # openssl rand -base64 32
REPORT_SALT=<lange Zufallszeichenkette>
```

Ohne `MODERATION_TOKEN` ist die Warteschlange **geschlossen**, nicht offen — ein
fehlendes Token bedeutet nie, dass keine Prüfung stattfindet. Die Moderation liegt unter
`/moderation`; das Token wird dort eingegeben und nirgends gespeichert.

`REPORT_SALT` schlüsselt den Wert, der statt der Absenderadresse gespeichert wird. Die
Adresse selbst wird nie gespeichert — nur ein damit berechneter Prüfwert, der ohne das
Salt wertlos ist und allein dazu dient, „derselbe Absender nochmal" von „jemand anders"
zu unterscheiden. Bleibt das Salt leer, wird bei jedem Start ein zufälliges erzeugt; das
funktioniert, setzt aber die Begrenzung pro Absender bei jedem Neustart zurück.

Gemeldet werden dürfen höchstens zehn Stellen pro Absender und Tag.

> **Vor dem öffentlichen Betrieb:** Wer Meldungen entgegennimmt, verarbeitet
> Nutzereingaben und braucht Impressum und Datenschutzerklärung. Beides fehlt noch.

## 5. Aktualisieren

```bash
git pull                                              # Compose-Datei und regions.yaml
docker compose -f deploy/docker-compose.yml pull      # neue Images
docker compose -f deploy/docker-compose.yml up -d
```

Der Checkout wird weiterhin gebraucht, aber nur noch für die Compose-Datei und
`regions.yaml` — der Programmcode kommt aus der Registry.

Wer nicht jedem Stand von `main` folgen will, trägt in der Konfigurationsdatei ein
`IMAGE_TAG=sha-<kurzer Hash>` ein. Dann ist eine Aktualisierung eine bewusste Änderung
dieses Werts statt dessen, was zwischenzeitlich gepusht wurde.

Migrationen laufen beim Start automatisch. Wenn sich die Bewertung oder das Clustering
geändert hat, müssen die Schwerpunkte neu berechnet werden:

```bash
docker compose -f deploy/docker-compose.yml run --rm importer hotspots
```

Neue Berichtsjahre des Unfallatlas erscheinen ungefähr im Juli. Der Importer lädt bereits
vorhandene Archive nicht erneut herunter, sondern fragt den Server nur, ob sie sich
geändert haben:

```bash
docker compose -f deploy/docker-compose.yml run --rm importer accidents -years 2016-2026
docker compose -f deploy/docker-compose.yml run --rm importer hotspots
```

## 6. Sichern

Alles Importierte lässt sich jederzeit neu erzeugen, eine Sicherung spart nur die
Importzeit. Sobald es Meldungen von Eltern gibt, ändert sich das — die sind nirgendwo
sonst vorhanden.

```bash
docker compose -f deploy/docker-compose.yml exec -T postgres \
  pg_dump -U ssk ssk | gzip > sicherung-$(date +%F).sql.gz
```

Für den Landkreis Zwickau sind das etwa 3 MB. Wer in `deploy/.env` einen anderen
`POSTGRES_USER` oder `POSTGRES_DB` gesetzt hat, muss die beiden `ssk` hier ersetzen.

Zurückspielen:

```bash
gunzip -c sicherung-2026-09-20.sql.gz | \
  docker compose -f deploy/docker-compose.yml exec -T postgres psql -U ssk ssk
```

## 7. Beenden

```bash
# Anhalten, Daten behalten
docker compose -f deploy/docker-compose.yml down

# Anhalten und alles löschen, auch die Datenbank und den Download-Cache
docker compose -f deploy/docker-compose.yml down -v
```

## Wenn etwas klemmt

**`port is already allocated`** — Port 8080 ist belegt. Einen anderen `API_PORT` in der
Konfigurationsdatei eintragen und erneut starten.

**Jede Seite antwortet mit 404** — der Reverse Proxy zeigt auf `API_PORT` statt auf
`WEB_PORT`. Die API kennt nur `/api/…` und `/healthz`, für die Seite selbst hat sie keine
Route. Sie sagt das inzwischen auch:

```bash
curl -s localhost:8080/ | jq -r .hint
# This port serves the API: /api/... and /healthz. The website is served by the web
# container on WEB_PORT — a reverse proxy belongs there, not here.
```

Im Proxy Host also `8090` statt `8080` eintragen, siehe Abschnitt 4a.

**Der Proxy erreicht die API nicht** — meist zeigt `API_BIND` auf eine Adresse, unter der
die Proxy-Maschine nicht herankommt. Mit
`ss -tlnp | grep <API_PORT>` prüfen, auf welcher Adresse tatsächlich gelauscht wird.

**`api` wird nicht `healthy`** — Logs ansehen:
`docker compose -f deploy/docker-compose.yml logs api`. Meist erreicht die API die
Datenbank nicht; dann auch `logs postgres` prüfen.

**Die Suche findet nichts, auch bekannte Schulen nicht** — dann ist der OSM-Import nicht
gelaufen. Die Startseite sagt das inzwischen selbst („Es sind noch keine Daten
importiert"), statt wie bei einem Tippfehler zu antworten. Nachzählen lässt es sich so:

```bash
docker compose -f deploy/docker-compose.yml exec -T postgres psql -U ssk -d ssk \
  -c "select (select count(*) from institutions) as einrichtungen,
             (select count(*) from accidents) as unfaelle,
             (select count(*) from hotspots) as schwerpunkte;" \
  -c "select source, detail, status, rows_written, finished_at
        from import_runs order by started_at desc limit 5;"
```

Für den Landkreis Zwickau stehen dort 443, 8332 und 1426. Fehlt in `import_runs` eine
Zeile `osm … succeeded`, dann Abschnitt 3 nachholen.

**Schwerpunktliste ist leer, Unfälle sind aber da** — `importer hotspots` wurde nicht
ausgeführt. Er läuft nicht automatisch, weil er die Tabelle vollständig ersetzt.

**Eine Schule hat keine Unfälle im Umkreis** — das ist oft einfach so. Im Landkreis
Zwickau haben 154 von 382 benannten Einrichtungen keinen Unfall mit Personenschaden im
500-Meter-Umkreis. Das ist ein Ergebnis, kein Fehler.

**`overpass is busy`** — der Dienst ist ausgelastet. Später erneut versuchen.

**`denied` oder `unauthorized` beim `pull`** — die Pakete stehen noch auf privat. Siehe
den Kasten in Abschnitt 2.

## Was fehlt, bevor das öffentlich laufen darf

- TLS und ein vorgelagerter Webserver
- Impressum und Datenschutzerklärung — bei einem öffentlich erreichbaren Angebot in
  Deutschland Pflicht
- die Prüfungen aus #20, die genau dieses Deployment automatisch testen

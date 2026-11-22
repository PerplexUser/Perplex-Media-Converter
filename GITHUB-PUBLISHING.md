# Veröffentlichen: GitHub und Homepage

## GitHub

1. Neues Repository für Perplex Media Converter anlegen.
2. Das **GitHub-Source-1.1.6.zip** entpacken und den Inhalt des Ordners
   `Perplex-Media-Converter` ins Repository hochladen/committen. Auch `.github`
   und `.gitignore` übernehmen. Die vorhandene MIT-Lizenz beibehalten.
3. Release mit Tag `v1.1.6` anlegen; Beschreibung aus
   `GITHUB-RELEASE-1.1.6.md` verwenden.
4. Installer, Portable-ZIP, Offline Developer Kit und **unbedingt**
   `Perplex-FFmpeg-Source-1.1.6.zip` als Release-Dateien anhängen.
   `SHA256SUMS.txt` ebenfalls anhängen. Die automatische GitHub-Quelldatei
   genügt nicht als FFmpeg-Quellpaket.
5. Die FFmpeg-Quelldatei im Beschreibungstext sichtbar neben den Downloads
   nennen/verlinken und ihren kostenlosen Download ohne Anmeldung prüfen.

## Homepage

`download.html` aus dem Veröffentlichungspaket zusammen mit der Installer-/
Portable-Datei, dem Anwendungs-Quell-ZIP, dem FFmpeg-Quellpaket und
SHA256SUMS.txt in **dasselbe öffentliche Downloadverzeichnis** kopieren.
`download.html` enthält funktionierende relative Links auf diese Dateinamen.
Alternativ diesen Downloadblock in die bestehende Homepage übernehmen.

**Der Quellcode-Link muss bei jedem Binärdownload sichtbar bleiben.**
Die FFmpeg-Quellen müssen dauerhaft entsprechend GPLv3 §6(d) verfügbar sein.
Der Nutzer muss sie nicht herunterladen; Installation und Betrieb sind offline.

Die alten 1.1.5-Installer/Developer-Kits nicht als diese korrigierte Ausgabe
anbieten. Eine Windows-Testinstallation und Deinstallation vor Freigabe ist
noch nötig; die Dateien sind nicht digital signiert (siehe TEST-REPORT.md).

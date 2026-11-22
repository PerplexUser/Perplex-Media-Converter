# Prüfbericht — Perplex Media Converter 1.1.6

Stand: 22. November 2026. Zielplattform: Windows 10/11 x64.

## Erfolgreich geprüft

- Anwendung, Installer, grafischer Offline-Builder und Kommandozeilen-Packager
  mit Go 1.27.4 für Windows/amd64 gebaut; `go vet` für Windows ohne Befund.
- 14 Go-Tests plus zehn Konvertierungs-Untertests bestanden, keine übersprungen.
  Die Funktionstests liefen unter Linux mit FFmpeg 7.1.5, **nicht** mit den
  Windows-Binärdateien. Geprüft: alle zehn Konvertierungspfade, Vorschau als
  H.264/AAC, externe Audiospur, Audioauswahl und Dateidialog-Skriptbehandlung.
- FFmpeg 9.1, x264 und LAME 3.100 aus den archivierten Quellen erfolgreich mit
  MinGW-w64 für Windows x64 gebaut. Die aktivierte Konfiguration enthält
  libx264, libmp3lame, AAC sowie die benötigten nativen Decoder/Container.
- Keine Änderungen an den ursprünglichen Dateien der drei Medien-Quellarchive
  festgestellt; vollständiger Bytevergleich gegen die archivierten Eingaben.
- FFmpeg-Release-Signatur gegen den offiziell veröffentlichten Fingerabdruck
  FCF986EA15E6E293A5644F10B4322F04D67658D8 geprüft.
- Beide Medienprogramme importieren ausschließlich normale Windows-DLLs:
  bcrypt.dll, KERNEL32.dll, msvcrt.dll und SHELL32.dll. Keine zusätzliche
  Drittanbieter-DLL muss nachgeladen oder installiert werden.
- Installer-Overlay vollständig gelesen, ZIP-CRC und beide FFmpeg-SHA-256-Werte
  geprüft. Uninstaller und eingebettete Anwendung stimmen mit den gebauten
  Dateien überein; alle Lizenzdateien sind im Installations-Payload enthalten.
- Quell-ZIP-Prüfsumme, Quellen-Prüfsummen und Zuordnung zu den eingebetteten
  ausführbaren Dateien geprüft.
- Packager lehnt die ursprünglichen 1.1.5-Gyan-Binärdateien ab und erzeugt dabei
  keine veröffentlichbare Installer-Datei. Abbruch- und Integritätsfälle getestet.

## Noch auf einem Windows-PC prüfen

Eine echte Windows-Ausführung ist in dieser Umgebung nicht verfügbar.
Ein Wine-Versuch konnte bereits ein minimales Testprogramm nicht starten.
Daher sind GUI, neuer Windows-FFmpeg-Build, Setup, Upgrade und Deinstallation
hier **nicht als zur Laufzeit getestet** ausgewiesen. Die EXEs sind unsigniert.

1. Vorhandene Anwendung schließen. Auf Windows 10/11 x64 installieren,
   idealerweise zunächst in einem Testbenutzerkonto. Netzwerkverbindung kann
   dafür getrennt sein. Startmenüeintrag und Lizenzdateien prüfen.
2. `Test-Offline.ps1` aus dem entpackten GitHub-Quellpaket oder Developer Kit
   ausführen. Es prüft die Binärhashes und alle zehn Konvertierungen mit
   mitgelieferten synthetischen Testdateien. Standardziel ist die Installation:

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File .\Test-Offline.ps1
   ```

   Für die Portable-Ausgabe `-BinDir "C:\Pfad\zur\Portable-Ausgabe"` ergänzen.
3. In der GUI Vorschau, Tonspurauswahl, externe Audiodatei, Unicode-Dateinamen
   und Abbrechen einer Konvertierung ausprobieren.
4. App schließen und über Windows-Einstellungen deinstallieren; Startmenü-
   eintrag und Programmdateien prüfen. Vorhandene Mediendateien müssen bleiben.

Diese Windows-Prüfung ist eine verbleibende Produktprüfung. Der beigelegte
Quellcode und die Lizenzhinweise beseitigen die zuvor dokumentierte fehlende
Quellenzuordnung für die **neu gebauten** FFmpeg-Dateien; die öffentlichen
Quell-Downloads müssen beim Veröffentlichen tatsächlich mit angeboten werden.

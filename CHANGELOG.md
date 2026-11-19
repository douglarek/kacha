# Changelog

## [1.2.0] - 2026-11-19

A film is a press of a key away, and its tray says where it stands.

- The recording key of the settings starts a film as well as ends it: a press
  starts a film, what the drag picks, and a press while one is made ends it.
  The row that picks the key says so, and so does the line under it.
- A film has an icon in the tray of its own, a red dot with the square of a
  stop button, up before the area and the screen of the desktop are picked:
  a click on it ends the film, and a film a terminal made has the icon too.
  The balloon of a film says the tray stops it as well as the key.
- A film says where it went in a balloon from either launch, a film the
  desktop turns down says so, and the key that ends one before it filmed says
  that without an error. The key waits for the file of the film rather than
  for its process, which a stopped film answers before it is reaped.
- The key of a terminal no longer reaches the recorder, which has a process
  group of its own: an interrupt ended it before it filmed. A film is stopped
  with SIGUSR1, which quits no app, and the launch of a film hears the key
  itself and quits once the film is written.
- MyGo 0.10.0.

## [1.1.0] - 2026-11-17

A film of the screen, on every system it captures.

- Record the screen, from a key that starts the film and the same key that
  ends it, or from the tray. A Wayland session records what its desktop
  shares through the portal, cropped to the area that was picked; macOS
  records with `screencapture`; a film is H.264 in an MP4 where the system
  has the encoder, and VP9 in a WebM where it has that one.
- The film says where it went in a notification, and a click on it opens
  the folder, with the film selected where the file manager does that.
- The desktop of a Wayland session names the app in the picker it shows for
  a film, which it learns from the id the app registers before its first
  call of the portal, and the film stops on its own signal, so that the
  notification of where it went is shown.
- MyGo 0.9.2, and the builds of CI and of the releases take the latest
  stable Go, whose standard library closes a vulnerability the scan of the
  build found.

## [1.0.1] - 2026-11-08

- The Windows builds come in arm64 as well as amd64, and a release holds
  their installers alone, named after their version and architecture.

## [1.0.0] - 2026-11-08

First release.

- Capture the screen of a Wayland session through the desktop portal, of
  macOS through `screencapture`, and of Windows through its own capture.
- Drag a part of the capture, annotate it with a rectangle, an ellipse, an
  arrow, a pen, a text, a number, a mosaic or an emoji, and copy it to the
  clipboard or save it.
- Crop what is kept, and drop the shapes the crop leaves out.
- Pick a text again to change it, and drag it where it belongs.
- Read the app's version, its name in Chinese and its updates from
  `mygo.json`, and update itself from the releases this repository signs.

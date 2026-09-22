# Shared drawing board (fork feature)

This fork of [m1k1o/neko](https://github.com/m1k1o/neko) adds a **shared drawing
board**: anyone in the room can draw on top of the streamed screen and every
viewer sees the strokes in real time, regardless of who holds the controls.

## Using it

- Click the **pen icon** in the top-right menu of the video (next to fullscreen).
- While draw mode is on, the mouse/touch draws instead of controlling the
  remote browser. Click the pen again, press **Esc**, or use the **X** in the
  toolbar to go back to normal control.
- Toolbar (top-left of the video):
  - colour swatches and three pen sizes
  - **ghost**: toggles *vanishing ink*. Strokes drawn with it disappear after a
    few seconds and are never stored (laser pointer style).
  - **undo** (or Ctrl/Cmd+Z): removes *your* last stroke. Admins can undo
    anyone's stroke by id via the API.
  - **trash**: clears the whole board for everyone.
- Late joiners receive the current board when they connect.
- Strokes from users you have *ignored* are not rendered for you.

## Configuration

All flags can also be given as env vars with the `NEKO_` prefix
(`NEKO_DRAW_ENABLED=false`).

| Flag                     | Default | Description                                   |
| ------------------------ | ------- | --------------------------------------------- |
| `draw.enabled`           | `true`  | enable the plugin                             |
| `draw.max_strokes`       | `2000`  | persisted strokes kept; oldest are dropped    |
| `draw.max_points`        | `4000`  | max points per stroke                         |
| `draw.clear_admin_only`  | `false` | only admins may clear the board               |

Per-user / global permission, like the chat plugin, through the `plugins` map
of the session settings or a member profile:

```yaml
member:
  multiuser:
    user_profile:
      plugins:
        draw.can_draw: false   # viewers can see but not draw
```

Admins always keep `can_draw` unless their own profile disables it.

## Protocol

Coordinates are normalised to the video frame (`0..1`), stroke width is a
fraction of the video width, so every viewer renders the same picture at any
window size.

| Event         | Direction        | Payload                                                |
| ------------- | ---------------- | ------------------------------------------------------ |
| `draw/init`   | server → client  | `{enabled, can_draw, strokes[]}` on connect            |
| `draw/stroke` | client → server  | `{id, color, width, fade?, points[[x,y],…]}`; repeat the same `id` to append points while drawing |
| `draw/stroke` | server → others  | same plus `user_id`                                    |
| `draw/undo`   | client → server  | `{id?}` (empty = my last stroke)                       |
| `draw/undo`   | server → all     | `{id, user_id}`                                        |
| `draw/clear`  | both             | `{}` / `{user_id}`                                     |

REST: `GET /api/draw/` returns the board, `DELETE /api/draw/` (admin) clears it.

## Code map

- `server/internal/plugins/draw/` — Go plugin: validation, permissions,
  in-memory board (`board.go`), websocket + REST handlers.
- `client/src/store/draw.ts` — Vuex module (board state, tool settings).
- `client/src/components/draw.vue` — canvas overlay + toolbar.
- `client/src/components/video.vue` — mounts the overlay and the pen button.
- `client/src/neko/{events,messages,index}.ts` — protocol wiring.

## Building the image

`Dockerfile.overlay` builds the server and client of this fork and copies them
over an upstream app image, so the runtime (xorg, gstreamer, browser) does not
have to be rebuilt:

```sh
docker build -f Dockerfile.overlay -t ghcr.io/shiritaicrm/neko/firefox:latest .
```

The `Fork Image` GitHub workflow does the same on every push to `master` and
publishes `ghcr.io/shiritaicrm/neko/firefox:latest`. Keep the upstream base
tag in sync with the upstream commit this fork is rebased on.

## Keeping up with upstream

```sh
git remote add upstream https://github.com/m1k1o/neko.git
git fetch upstream
git merge upstream/master
```

The server plugin lives in its own directory and merges cleanly. Expect small
conflicts in `client/src/components/video.vue` and `client/src/neko/index.ts`.

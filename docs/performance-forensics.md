# Cadence — Análisis forense de tamaño y consumo

**Fecha:** 2026-11-12
**Commit analizado:** `fe2ec34` (`fix(deps): resolve minimatch and deepmerge npm advisories`)
**Entorno de medición:** Arch Linux, kernel Linux, Wayland; GTK 4.24.1; WebKitGTK 6.0 **2.54.2**; Go 1.26; Wails v3.0.0-beta.34.
**Alcance:** tamaño de los artefactos publicados, huella en disco, memoria y CPU en runtime, dependencias y trabajo innecesario. No se modificó código de la aplicación; este documento es el único artefacto nuevo.

---

## 1. Resumen ejecutivo

Cadence no tiene un problema de código Go: el heap medido es de **4 MB** con **18 goroutines**. Tiene un problema de **plataforma y de arquitectura de runtime**: se envía un AppImage que incluye una pila WebKit/GTK completa, y en el modo por defecto conviven **tres runtimes** (WebKitGTK para la UI, Electron/Chromium para Spotify, y go-librespot para el catálogo).

**Aclaración clave:** Electron **no** está dentro del AppImage. El AppImage incluye WebKitGTK/GTK para la UI; Electron/ECS se descarga en el primer arranque (117 MB) y se extrae en `~/.cache/cadence/webplayer/runtime` (283 MB). Por lo tanto, quitar Electron reduce la **huella instalada**, la **descarga inicial** y la **RAM**, pero **no el tamaño del AppImage**. Para eso hay que dejar de bundelear WebKit/GTK (sección 4.3) o publicar `.deb`/`.rpm`.

Los cinco números que importan:

| Métrica | Valor medido | Comentario |
|---|---:|---|
| AppImage publicado | **115.5 MB** (~110 MiB) | El 86 % es pila de sistema (WebKitGTK, JSC, ICU, GTK, códecs) |
| Huella instalada tras primer arranque | **~540 MB** | AppImage + runtime Electron 283 MB + perfiles 106 MB + Soloist 35 MB |
| Memoria en modo web, reproducción detenida | **725 MB PSS** | WebKit UI 225 + Electron 357 + go-librespot 28 + Cadence 115 |
| Memoria en modo demo, ventana real 840×440 | **432 MB PSS** | WebKitWebProcess solo: 279 MB PSS |
| CPU de la UI (software rendering) | **~39 % de un core en reposo** (ventana 1280×720); **~100 % reproduciendo** (840×440) | Animaciones CSS infinitas + blurs + ventana transparente |

**Las tres acciones de mayor impacto, en orden:**

1. **Publicar `.deb`/`.rpm` (o un AppImage “lean”) que use el WebKit del sistema.** Medido: el mismo AppDir sin WebKitGTK/JSC/ICU y sin códecs de video comprime a **39.8 MB** con zstd-19 y bloque de 1 MB, frente a **104.6 MB** del AppDir completo con esa misma configuración (113.3 MB con el bloque por defecto de 128 KB). Un paquete `.deb`/`.rpm` ronda los **17 MB**. El AppImage “portable” de 115.5 MB puede seguir existiendo como opción para distros viejas.
2. **Reemplazar el runtime Electron por el browser del sistema (Chrome/Brave/Chromium) en `--headless=new`, manteniendo `web` como default.** Elimina la descarga de 117 MB, los 283 MB en disco y ~95 MB PSS, sin perder el SDK ni Widevine. PoC validado en este equipo: `WIDEVINE_OK` + audio real (sección 12). Fallback a ECS si no hay browser compatible. El modo `librespot` queda descartado como default para cuentas nuevas: Spotify cerró los endpoints privados (sección 12.2).
3. **Apagar el repintado continuo cuando no hay reproducción.** Hay **40 declaraciones `animation: ... infinite`**, **18 `backdrop-filter`** y **42 `blur()`**; varias animaciones corren aunque el player esté detenido. En la medición, la UI detenida seguía consumiendo ~39 % de un core.

---

## 2. Metodología

### 2.1 Qué se midió

- Tamaños reales de `bin/cadence`, `bin/go-librespot`, `bin/cadence.AppImage` y `bin/*.tar.gz`.
- Composición del AppDir generado por `task package` (`build/linux/appimage/build/cadence-x86_64.AppDir`).
- Reproducción de la compresión con `mksquashfs` para comparar escenarios sin rebuildear la app.
- Procesos, RSS y **PSS** (`/proc/<pid>/smaps_rollup`) en un compositor Wayland headless (`sway` con backend `headless`), sin abrir ventanas en el escritorio del usuario.
- Perfil Go vía `--diagnostics`: `http://127.0.0.1:<port>/debug/pprof/heap` y `goroutine`.
- Lectura de código y configuración para atribuir cada costo.

### 2.2 Condiciones y límites

- Las mediciones de CPU se hicieron con **renderizado por software** (sway headless usa pixman; no hay GPU real en el compositor anidado). En el escritorio real con GPU, el costo de CPU baja, pero el trabajo de composición sigue existiendo. Los números de CPU deben leerse como **comparativos**, no como el consumo exacto en el equipo del usuario.
- La medición “modo web detenido” se hizo con una ventana de 1280×720 porque sway la tileó. La medición “demo” se hizo con la ventana real de 840×440 (la que pide `main.go`). Se indica en cada caso.
- PSS reparte las páginas compartidas entre procesos; la suma de RSS es mucho mayor (~1.7 GB en modo web) y no representa memoria física real.

### 2.3 Reproducir las mediciones

```bash
# Tamaño y composición
stat -c '%s %n' bin/cadence bin/go-librespot bin/cadence.AppImage bin/*.tar.gz
du -sh build/linux/appimage/build/cadence-x86_64.AppDir/usr/lib/*

# Composición del binario
size -A bin/cadence

# Perfil Go (sin GPU, modo demo)
XDG_RUNTIME_DIR=/tmp/cadence-xdg WAYLAND_DISPLAY=wayland-1 \
  ./bin/cadence --demo --diagnostics
curl -s http://127.0.0.1:<puerto>/debug/pprof/heap -o heap.pb.gz
go tool pprof -top -inuse_space heap.pb.gz

# Memoria real por proceso
for p in $(pgrep -x cadence) $(pgrep -f WebKitWebProcess); do
  awk '/^Pss:/{s+=$2} END{printf "%s %.0f MB\n", FILENAME, s/1024}' /proc/$p/smaps_rollup
done
```

---

## 3. Mapa de procesos

En el modo por defecto (`playback.backend = web`), al arrancar Cadence conviven:

```
┌─────────────────────────────────────────────────────────────────┐
│ Cadence (Go + Wails)                                            │
│  ├─ WebKitWebProcess  ← UI Svelte (GTK4 / WebKitGTK 6.0)        │
│  ├─ WebKitNetworkProcess                                        │
│  ├─ go-librespot      ← sesión de catálogo / biblioteca         │
│  └─ Electron 44 (castlabs ECS)                                  │
│       ├─ browser process                                        │
│       ├─ GPU process                                            │
│       ├─ network utility                                        │
│       ├─ media utility                                          │
│       ├─ renderer (Spotify Web Playback SDK)                    │
│       └─ zygotes                                                │
└─────────────────────────────────────────────────────────────────┘
```

En modo `soloist`, el lugar de Electron lo ocupa el binario Soloist (~35 MB en disco). En modo `librespot`, go-librespot hace playback y catálogo, y **no hay Electron ni Soloist**.

Puntos clave del arranque (`main.go`, `internal/wailsapi/audio/service.go`):

- `engine.NewSupervisor(...)` y `supervisor.Start(ctx)` se ejecutan **siempre** que `enabled` sea true, incluso en modo web o Soloist (`service.go:121-125`). En esos modos go-librespot queda como “Cadence Library”, sin MPRIS y sin cache de audio, pero sigue siendo un cliente Spotify Connect completo.
- En modo web, `provisionWeb` descarga/instala el runtime, hace login y arranca Electron **automáticamente al inicio** (`service.go:126-139`, `internal/wailsapi/audio/web.go:99-133`).
- El servicio `developer` (automatización headless del dashboard de Spotify) se construye siempre en modo no-demo, pero no lanza procesos hasta que se usa.

---

## 4. Tamaño de distribución

### 4.1 Artefactos actuales

| Artefacto | Bytes | Comentario |
|---|---:|---|
| `cadence.AppImage` | 115,546,616 | Artefacto principal de release |
| `cadence-0.2.0-linux-x86_64.tar.gz` | 16,204,244 | Cadence + go-librespot + skins |
| `bin/cadence` | 18,113,408 | Go + Wails + frontend embebido |
| `bin/go-librespot` | 18,167,616 | Sidecar, `-s -w`, enlaza mpg123/FLAC/vorbis/ogg/ALSA |
| `bin/cadence.zip` | 114,941,599 | Artefacto local viejo, no lo publica el workflow |
| `build/.../cadence-x86_64.AppDir` | ~324 MB | Intermedio de build (gitignored) |

El repositorio completo pesa **803 MB**, casi todo `build/` (363 MB), `bin/` (270 MB) y `frontend/node_modules` (152 MB). No afecta al usuario final, pero sí al disco de desarrollo y a la caché de CI.

### 4.2 Anatomía del AppImage

El AppDir tiene 324 MB; `usr/lib` solo son 289 MB:

| Componente | Tamaño sin comprimir | Contribución estimada al AppImage (zlib -9) |
|---|---:|---:|
| `libwebkitgtk-6.0.so.4` | 94 MB | 39.6 MB |
| `libjavascriptcoregtk-6.0.so.1` | 40 MB | 14.2 MB |
| `libicudata.so.78` + `libicuuc` + `libicui18n` | 37 MB | 15.2 MB |
| Pila GTK (gtk4, pango, gdk-pixbuf, cairo, harfbuzz, gio…) | ~30 MB | 10.8 MB |
| Códecs de video / gstreamer (aom, SvtAv1, rav1e, jxl, dav1d, gst*) | ~30-40 MB | 11.5 MB |
| Crypto/TLS (crypto, gnutls, gcrypt, p11-kit, leancrypto) | ~20 MB | 6.0 MB |
| `usr/bin` (cadence + go-librespot) | 34 MB | 13.3 MB |
| Resto (iconos, docs, skins, glib schemas) | ~2 MB | 0.3 MB |

**Lectura:** WebKitGTK + JSC + ICU son **~69 MB comprimidos**, más de la mitad del AppImage. La pila GTK, códecs y crypto suman otros ~28 MB. Cadence y go-librespot juntos son solo ~13 MB comprimidos.

### 4.3 Escenarios de empaquetado medidos

Se reconstruyó el AppDir y se comprimió con `mksquashfs` para comparar. La compresión real del AppImage actual es zstd con bloque de 128 KB (113.25 MB de squashfs + ~2.3 MB de runtime = 115.5 MB), así que los valores son comparables:

| Escenario | squashfs | AppImage estimado | Qué incluye |
|---|---:|---:|---|
| Actual (gzip, referencia) | 126.8 MB | — | Todo |
| Actual (zstd-19, bloque 1 MB) | 104.6 MB | ~107 MB | Todo, mejor compresión |
| **Lean** (sin WebKit/JSC/ICU ni códecs de video; usa WebKit del sistema) | **46.5 MB** | **~49 MB** | GTK stack + app + go-librespot |
| Lean + zstd-19 bloque 1 MB | **39.8 MB** | **~42 MB** | Igual, mejor compresión |
| **Mínimo** (sin ninguna librería de sistema; estilo `.deb`) | **14.0 MB** | **~16 MB** | Solo `usr/bin` + `usr/share` |
| `.deb`/`.rpm` real (cadence + go-librespot + skins) | — | **~17 MB** | Depende de `webkitgtk-6.0`, `gtk4` |

**Conclusión:** el AppImage de 115.5 MB no es “el tamaño de Cadence”; es el precio de bundelear WebKit. Si se acepta depender de WebKitGTK 6.0 del sistema (Arch, Fedora 40+, Ubuntu 24.04+, Debian 13+), el artefacto baja a **~40-49 MB**. Si además se usa el formato nativo de la distro, baja a **~17 MB**.

### 4.4 Hallazgos de tamaño

#### S1 — WebKit/GTK bundeleado: el 86 % del AppImage

**Evidencia:** tabla 4.2; `wails3 generate appimage` copia WebKit y sus helpers a propósito, y parchea la librería para que arranquen desde el AppDir ([docs de Wails](https://wails.io/guides/build/linux/)). El propio `build/linux/Taskfile.yml` ya elimina la duplicación GTK3/WebKit2GTK 4.1, lo que confirma que el equipo ya peleó con esto.

**Impacto:** 115.5 MB → ~42 MB (lean) o ~17 MB (paquete nativo).

**Esfuerzo:** bajo para `.deb`/`.rpm` (Wails ya lo soporta); medio para un AppImage lean propio.

**Riesgo:** en distros sin WebKitGTK 6.0 el artefacto lean no arranca. Se mitiga publicando ambos: “portable” y “lean”, con dependencia documentada.

#### S2 — Copias duplicadas de la misma librería en el AppDir (no afectan al AppImage)

**Evidencia:** `usr/lib` tiene 157 archivos regulares y **0 symlinks**; `librsvg-2.so`, `.so.2` y `.so.2.63.4` son tres copias reales de 4.8 MB; `libgio` tres copias de 2.0 MB; `libpango`, `libpangocairo`, `libpangoft2`, `libgdk_pixbuf` y `libgobject` también. Total: **16.5 MB duplicados** en el AppDir.

**Matiz forense:** al comprimir con `mksquashfs`, el resultado con y sin deduplicar es **exactamente el mismo** (126,799,872 bytes en ambos casos): squashfs ya deduplica archivos idénticos. Por lo tanto, esto **no infla el AppImage publicado**; solo infla el AppDir local y cualquier extracción/instalación que copie archivos reales (por ejemplo, un futuro `.deb` construido desde ese AppDir). No es una optimización de tamaño de release; es higiene de build.

#### S3 — `NO_STRIP=1` forzado

**Evidencia:** `build/linux/Taskfile.yml` pasa `NO_STRIP=1` a linuxdeploy; Wails además lo activa automáticamente cuando detecta `.relr.dyn` ([docs](https://wails.io/guides/build/linux/): “AppImages will be slightly larger (~20-40%)”).

**Impacto real hoy:** bajo en Arch, porque sus librerías ya vienen stripped (`readelf -S` no muestra `.symtab` en `libwebkitgtk`). En un host de build con librerías con símbolos (por ejemplo, ciertas imágenes de CI), el impacto sería de decenas de MB.

**Acción:** no es prioritario, pero si se cambia el host de build, verificar con `file`/`readelf` y no forzar `NO_STRIP=1` salvo que sea necesario.

#### S4 — go-librespot viaja en todos los artefactos (18 MB)

**Evidencia:** `build/linux/Taskfile.yml` copia `bin/go-librespot` al AppImage y al tarball. En modo web y Soloist se usa solo como sesión de catálogo (`libraryOnly`), sin audio.

**Impacto:** ~6.6 MB comprimidos en el AppImage; 18 MB en disco.

**Nota de correctitud:** `go-librespot` enlaza `libasound.so.2`, pero el AppDir **no** la incluye (linuxdeploy la trata como librería base). En un host sin ALSA el sidecar no arranca. Es una dependencia del sistema no declarada; conviene documentarla o empaquetarla.

**Opciones:**
- Dejarlo (offline-friendly, simple). Es la opción actual y es defendible.
- Auto-descargarlo en el primer uso, como ya se hace con Soloist (~10 MB). Ahorra 6.6 MB en el release, pero agrega un modo de fallo de red en el primer arranque.
- Eliminarlo en modo web reemplazando catálogo/biblioteca por Spotify Web API. Ahorra el proceso entero (28 MB PSS) pero es un refactor grande.

#### S5 — Fuentes: 2.1 MB de los 2.7 MB del frontend embebido

**Evidencia:** `frontend/dist/fonts` = 2,136,540 bytes; `frontend/dist/assets` = 567,130 bytes.

| Fuente | Actual | WOFF2 | Ahorro |
|---|---:|---:|---:|
| `Twemoji.Mozilla.ttf` | 1,474,284 | 493,000 | 981 KB |
| `JetBrainsMonoVariable.ttf` | 300,144 | 113,820 | 186 KB |
| `InterVariable.woff2` | 352,240 | ya es woff2 | — |

**Acción inmediata:** convertir JetBrains a WOFF2 y Twemoji a WOFF2 (medido con `woff2_compress`). Ahorro: **1.16 MB** en disco y en el binario; ~0.5 MB en el AppImage comprimido.

**Acción mejor:** Twemoji se usa solo como fallback de emojis en nombres de canciones/artistas. En Linux, `Noto Color Emoji` suele estar instalado; se puede quitar Twemoji y confiar en el sistema, o subsetear con `pyftsubset` a los emojis realmente usados. Si se quiere cobertura total, WOFF2 es el mínimo razonable.

#### S6 — Artefactos locales obsoletos

**Evidencia:** `bin/cadence.zip` (114.9 MB) es un artefacto local viejo que el workflow de release no publica; `bin/` pesa 270 MB.

**Acción:** `task clean` antes de medir/publicar; no es un problema del usuario final, pero contaminó el análisis inicial y puede confundir a quien mire el repo.

#### S7 — Compresión del AppImage: margen pequeño

**Evidencia:** appimagetool usa zstd por defecto con `-b 128K` ([referencia](https://github.com/appimage/appimagetool)); el AppImage actual ya está en zstd. Con `-b 1M` y nivel 19, el mismo contenido baja de 113.25 MB a **104.6 MB** (~8 %). Wails no expone `--mksquashfs-opt`, así que requiere invocar appimagetool directamente después de `wails3 generate appimage`.

**Acción:** solo vale la pena si se mantiene el AppImage portable. No cambia la ecuación frente a “no bundelear WebKit”.

#### S8 — `cadence` y `go-librespot` ya están bien compilados

`-trimpath -buildvcs=false -ldflags="-w -s"` y `-tags production` están aplicados. `bin/cadence` está stripped; `size -A` muestra 7.0 MB `.text`, 3.6 MB `.rodata` (incluye 2.7 MB de frontend), 5.4 MB `gopclntab` y 1.5 MB `.go.type`. No hay un recorte fácil y seguro aquí. **UPX no se recomienda**: comprime a ~7 MB pero descomprime en RAM al arrancar, rompe core dumps y empeora el tiempo de inicio; para un player de música no compensa.

---

## 5. Consumo en runtime

### 5.1 Mediciones

**Modo web (por defecto), reproducción detenida, ventana 1280×720, PSS:**

| Grupo | Procesos | PSS |
|---|---:|---:|
| Cadence (Go + GTK/WebKit en proceso) | 1 | 115 MB |
| WebKit UI (`WebKitWebProcess` + network + bwrap) | 4 | 225 MB |
| Electron helper (browser, GPU, media, network, renderer, zygotes) | 9 | 357 MB |
| go-librespot | 1 | 28 MB |
| **Total** | **15** | **725 MB** |

RSS sumado: ~1.7 GB. El heap Go es de **4 MB** y hay **18 goroutines**; el costo no está en Go.

**Modo demo, ventana real 840×440, reproduciendo, software rendering:**

| Proceso | CPU | PSS |
|---|---:|---:|
| `cadence --demo` | 11 % | 121 MB |
| `WebKitWebProcess` | **100 % de un core** | **279 MB** |
| `WebKitNetworkProcess` | 0 % | 32 MB |
| **Total** | — | **432 MB** |

**Modo web detenido (medición anterior, 1280×720):** `WebKitWebProcess` ~39 % de un core. Reproduciendo en modo demo (840×440) subió a ~100 %. La diferencia mezcla estado de reproducción y tamaño de ventana; aun así, el salto muestra cuánto agregan el visualizador y las animaciones de reproducción.

### 5.2 Hallazgos de runtime

#### R1 — Tres runtimes en el modo por defecto

**Evidencia:** sección 3. En modo web se ejecutan WebKitGTK (UI), Electron/Chromium (playback) y go-librespot (catálogo).

**Impacto:** 725 MB PSS en reposo; 15 procesos; tres actualizadores de estado en paralelo.

**Opciones (con la restricción de cuentas nuevas):**
- **Mantener `web` como default** (requisito del producto). El costo de Electron se reduce reemplazándolo por el browser del sistema en `--headless=new` (sección 12), no cambiando a `librespot`.
- **No arrancar Electron hasta el primer playback** (hoy `provisionWeb` corre al inicio): elimina la descarga y el proceso en usuarios que solo browsean o usan otro backend.
- **No arrancar go-librespot** hasta que se abra Browse/Library; hoy arranca siempre.
- **Lite mode con `librespot`** queda solo para cuentas antiguas: Spotify cerró los endpoints privados y las cuentas creadas después de ~noviembre 2025 no funcionan (sección 12.2). No puede ser el default.

#### R2 — Electron: 283 MB en disco + 47 MB de perfil + 42 MB duplicados de Widevine

**Evidencia:** `~/.cache/cadence/webplayer/runtime` = 283 MB; `electron` solo = 220 MB; `LICENSES.chromium.html` = 20 MB; `resources.pak` = 13 MB; `icudtl.dat` = 11 MB; `locales` = 8.8 MB (55 archivos); `libvk_swiftshader.so` = 4.5 MB. Perfil = 47 MB, con `WidevineCdm` 21 MB y `component_crx_cache` 21 MB. `~/.config/cadence/developer/profile` repite **otros** 21 + 21 MB de Widevine/crx: **42 MB duplicados** entre los dos perfiles Electron.

**Acciones:**
- Podar `locales/*` a `en-US.pak` (y el idioma que se quiera soportar): ~8.5 MB.
- Evaluar quitar `libvk_swiftshader.so` si no se necesita fallback Vulkan software: 4.5 MB (riesgo bajo/medio).
- `LICENSES.chromium.html`: 20 MB; revisar obligaciones de licencia antes de tocarlo. Si se puede, comprimirlo o enlazarlo.
- Compartir el perfil entre developer y webplayer, o borrar `developer/profile/{WidevineCdm,component_crx_cache}` después del onboarding: ~42 MB.
- Total realista en disco: **~55 MB menos** sin tocar funcionalidad (hasta ~75 MB si además se puede comprimir o eliminar el `LICENSES.chromium.html`).

#### R3 — La UI WebKit pesa 279 MB PSS para una ventana de 420×220 lógicos

**Evidencia:** medición demo 840×440. El heap Go no explica nada (4 MB). La UI es Svelte + canvas + CSS.

**Causas identificadas:**
- **18 `backdrop-filter`** y **42 `blur()`** en `frontend/src` y `skins-dev`.
- **40 declaraciones `animation: ... infinite`**; varias corren siempre: `aurora-drift` (`.clarity::before`, 28 s), `player-bloom` (12 s), `mark-breathe` (logo, 5.2 s), `provider-pulse` (2.4 s), `control-dot`, `breathe`, `ring-spin`, etc.
- Ventana **transparente** (`BackgroundTypeTransparent`) + `Zoom: 2.0` + `mix-blend-mode: screen` en el visualizador y en `player::after`.
- WebKitGTK 2.54 tuvo una regresión conocida con `backdrop-filter` (corregida en **2.54.2**, que es la versión instalada). Aun corregida, cada capa de blur es cara; el artículo de referencia documenta el caso: <https://linuxundich.de/en/webkitgtk-memory-leak-backdrop-filter/>.

**Acciones:**
- Gatear las animaciones decorativas con `.is-playing` (varias ya lo hacen; aurora/bloom/logo/transport no).
- Bajar radios de blur (30 px → 12-16 px) y reemplazar `backdrop-filter` por fondos semitransparentes opacos donde el efecto sea sutil. El propio artículo muestra que el cambio visual es mínimo.
- Ofrecer “fondo sólido” (sin transparencia) como opción; en Wayland la transparencia impide fast paths de scanout.
- Respetar `prefers-reduced-motion` también para las animaciones decorativas (hoy hay 5 bloques `@media`, pero no cubren todo).
- Medir con el protocolo de la sección 9; objetivo razonable: bajar el CPU de reposo a <10 % de un core en hardware con GPU y reducir el PSS de WebKit en >30 %.

#### R4 — El visualizador captura todo el audio del monitor con `parec`

**Evidencia:** `internal/visualizer/capture_linux.go` lanza `parec --format=s16le --rate=44100 --channels=1 --latency-msec=20 --device=@DEFAULT_MONITOR@` y analiza a 30 fps. Se activa solo mientras `playing && !reducedMotion && visible` (`Visualizer.svelte:265`), lo cual está bien.

**Impacto:** CPU moderado (FFT + 30 fps), pero captura **todo** el audio del sistema, no solo el de Cadence. Es un costo de privacidad además de CPU.

**Acciones:** bajar a 15-20 fps y 22050 Hz cuando el modo visualizador lo permita; documentar que captura el monitor; ofrecer desactivarlo por completo. No es prioritario frente a R3.

#### R5 — Polling del Web API cada 1 segundo

**Evidencia:** `internal/webplayer/provider.go:28` `webStreamPlayerInterval = time.Second`; `Stream` llama `api.Player(ctx)` cada segundo y `api.Devices` cada 10 s. Son ~86,400 requests/día con la app abierta. Además, `page.js` registra `player.addListener("player_state_changed", () => {})`: **el SDK ya entrega los cambios de estado y se descartan**.

**Acciones:**
- Reenviar `player_state_changed` por el WebSocket de control y usarlo como fuente principal; el polling queda como reconciliación cada 5-10 s.
- Si no se quiere tocar el helper, al menos hacer el intervalo adaptativo: 1 s reproduciendo, 5 s pausado, 15 s sin dispositivo.
- Impacto: menos red, menos CPU, menos batería; sin cambio visual (la UI ya interpola la posición).

#### R6 — Timers redundantes en la UI

**Evidencia:**
- `NowPlayingBar.svelte:58` actualiza `positionMs` cada **250 ms**.
- `NowPlayingView.svelte:68` actualiza `positionTick` cada **200 ms**.
- `Progress.svelte:67` corre un `requestAnimationFrame` a 60 fps mientras reproduce.
- `SettingsView.svelte:57` refresca estado web cada **3 s** (solo con la vista abierta).
- `SoloistSetupDialog.svelte:56` sondea cada **250 ms** (solo con el diálogo abierto).
- `music.svelte.ts:154` sondea pairing cada **5 s** (solo sin autenticar).

**Acción:** un único store de posición (el `Progress` ya tiene la interpolación) y eliminar los intervalos de 200/250 ms; o bajarlos a 1 s. Impacto: menos recálculo de estilos, menos batería. Bajo riesgo.

#### R7 — go-librespot siempre encendido

**Evidencia:** `service.go:121-125`. En modo web/Soloist se usa solo para catálogo (`libraryOnly`), sin MPRIS ni cache.

**Impacto:** 28 MB PSS + un cliente Connect extra + I/O de red en segundo plano.

**Acción:** arrancarlo bajo demanda (primera llamada a Playlists/Albums/Search/ContextTracks) y detenerlo tras un período de inactividad. Cambio acotado en `catalog.Service`/`audio.Service`. Mientras `web` sea el default, el sidecar sigue siendo necesario para catálogo; el objetivo es arrancarlo bajo demanda.

#### R8 — Caches: 64 MB de RAM + 256 MB de disco, y letras sin tope

**Evidencia:** `internal/artwork/cache.go:36-42`: `defaultMaxEntries=256`, `defaultMaxBytes=64<<20`, `defaultMaxImage=3<<20`, `defaultDiskLimit=256<<20`. `internal/lyrics/lyrics.go` escribe un JSON por track con TTL, **sin límite de tamaño ni poda**.

**Acción:** para una app de 840×440, 64 MB de RAM para artwork es generoso; 16-32 MB alcanza. El cache de disco de 256 MB también se puede bajar a 64-128 MB o exponerlo en Settings. Para letras, agregar LRU o poda por antigüedad (hoy crece indefinidamente). Impacto: menos memoria y menos disco; bajo riesgo.

#### R9 — Flags de Electron que impiden el ahorro en background

**Evidencia:** `internal/webplayer/process.go:104-113` pasa `--disable-background-timer-throttling`, `--disable-renderer-backgrounding` y `--disable-backgrounding-occluded-windows`. Es intencional para no cortar playback, pero mantiene el renderer a pleno ritmo aunque la ventana esté oculta.

**Acción:** permitir throttling cuando no hay reproducción (o cuando la ventana está oculta y el estado es “paused”). Impacto: menos CPU en reposo; riesgo: microcortes si se aplica mal. Medir.

#### R10 — La ventana transparente y el zoom 2× encarecen la composición

**Evidencia:** `main.go:392-398`: `Zoom: 2.0`, `Frameless: true`, `BackgroundTypeTransparent`, `WebviewGpuPolicyOnDemand`. En Wails v3 beta.34 con WebKitGTK 6.0, `OnDemand` se mapea a **ALWAYS** (la política `ON_DEMAND` ya no existe en WebKitGTK 6.0; ver `linux_cgo.go:1263-1272`).

**Acción:** evaluar `BackgroundTypeSolid` para la ventana principal y dejar la transparencia solo si el diseño la necesita. En Wayland, una superficie opaca permite scanout directo y evita blending de toda la ventana. Impacto: CPU/GPU y memoria; riesgo: se pierden bordes redondeados si se usaban.

#### R11 — El helper de developer es grande para un uso de una sola vez

**Evidencia:** `internal/developer` lanza un Electron headless por acción contra el dashboard; usa el mismo runtime que el webplayer, pero un `user-data-dir` distinto (`~/.config/cadence/developer`). Deja 59 MB permanentes, 42 MB de ellos duplicados de Widevine.

**Acción:** borrar el perfil del developer después de completar el onboarding, o compartir el perfil con el webplayer (con cuidado por el singleton de Chromium), o mover el onboarding a un flujo CLI/URL que no requiera Electron. Impacto: hasta 59 MB; esfuerzo medio.

#### R12 — Diagnóstico y logs: correctos

`--diagnostics` es opt-in y escucha solo en loopback (`main.go:487-502`). El log del engine se trunca al arrancar (`O_TRUNC`), el del webplayer tiene tail de 200 líneas, y los logs medidos pesan 12 KB. **No hay fuga de logs.** El cache de artwork está acotado; el de letras no (R8).

---

## 6. Lo que ya está bien (no tocar)

- **Build flags:** `-trimpath`, `-buildvcs=false`, `-ldflags="-w -s"`, `-tags production`. Binario stripped.
- **Frontend:** sin sourcemaps en `dist`; 567 KB de JS/CSS para una app con skins, browse, i18n y tests es razonable. `intl-messageformat` y `jsonschema` no son el problema.
- **Go:** sin fugas; 4 MB de heap y 18 goroutines en reposo. Los providers usan eventos + reconciliación, no polling ciego (salvo el web player, R5).
- **Caches de artwork:** bien diseñados (LRU, single-flight, límites). Solo falta ajustar los valores.
- **Seguridad:** CSP estricta, `media-src 'none'`, sandbox de Chromium activo, tokens fuera del renderer, logs redactados.
- **Empaquetado:** el Taskfile ya limpia WebKit2GTK 4.1/GTK3 y genera checksums; el release verifica la versión contra `build/config.yml`.

---

## 7. Alternativas investigadas

| Alternativa | Tamaño / costo | Ventajas | Desventajas | Veredicto |
|---|---|---|---|---|
| **`.deb`/`.rpm` con WebKit del sistema** | ~17 MB de paquete; ~37 MB instalado | Nativo, actualizaciones por distro, sin bundelear 100 MB | Depende de `webkitgtk-6.0` (Arch, Fedora 40+, Ubuntu 24.04+; no Debian 12) | **Recomendado como artefacto principal para distros modernas** |
| **AppImage “lean”** | ~40-49 MB | Un solo archivo, sin instalar dependencias | Requiere WebKitGTK 6.0 del sistema; menos portable que el actual | **Recomendado como segundo artefacto** |
| **AppImage actual (portable)** | 115.5 MB | Funciona en cualquier distro con glibc reciente | 86 % del peso es pila de sistema; arranque y RAM altos | Mantener solo para compatibilidad |
| **Browser del sistema (Chrome/Brave/Chromium) headless** | 0 MB extra; sin descarga | Reemplaza a Electron para el SDK; conserva Widevine y audio; ya está instalado | Depende del browser y su versión; CDM/opt-in; flags; no reduce el AppImage | **PoC validado en este equipo (sección 12); mejor opción para quitar Electron** |
| **Flatpak (Flathub)** | App ~17 MB; runtime `org.gnome.Platform` compartido | Sandbox, runtime compartido entre apps, actualizaciones delta | Runtime grande la primera vez; permisos/portales; curva de build | **Muy buena opción para distribución masiva** ([conceptos](https://docs.flatpak.org/en/latest/basic-concepts.html)) |
| **Lite mode con go-librespot** | 0 MB extra; -283 MB de runtime | Elimina Electron, Widevine y 357 MB PSS; ya está incluido | Spotify cerró los endpoints privados: **no funciona en cuentas nuevas** (post ~nov-2025) | Solo cuentas antiguas; no puede ser el default |
| **Soloist como único backend** | ~35 MB en disco | Audio oficial de Spotify, sin Electron | Requiere API key y onboarding; no sirve para catálogo | Buena opción “sin Electron” si se quiere audio oficial |
| **librespot (Rust) en lugar de go-librespot** | Binario similar (~10-20 MB) | Muy maduro, bajo consumo | Reemplaza parches propios; reescribir integración | No compensa el esfuerzo |
| **Tauri / reescritura nativa** | ~5-10 MB | Binario mucho menor | Reescritura completa de UI y skins; WebKit igual en Linux | No justificado |
| **UPX en `cadence` y `go-librespot`** | 18 MB → ~7 MB | Reduce el archivo | +RAM al arrancar, rompe core dumps, falsos positivos de AV | **No recomendado** |
| **Quitar Twemoji y usar Noto Color Emoji** | -1.5 MB | Menos peso | Depende de la fuente del sistema; look variable | Válido si se acepta el tradeoff |
| **Subsetear Twemoji** | -1.0 a -1.4 MB | Mantiene cobertura usada | Hay que definir el set; los nombres de canciones pueden traer cualquier emoji | Válido con WOFF2 como mínimo |

**Fuentes consultadas:**
- Wails, Linux Packaging (bundelea WebKit, `NO_STRIP` 20-40 %): <https://wails.io/guides/build/linux/>
- appimagetool, compresión zstd y `--mksquashfs-opt`: <https://github.com/appimage/appimagetool>
- Flatpak, runtimes compartidos: <https://docs.flatpak.org/en/latest/basic-concepts.html>
- WebKitGTK 2.54.2, fix de rendimiento con `backdrop-filter`: <https://webkitgtk.org/2026/10/23/webkitgtk2.54.2-released.html>
- Caso reproducible de `backdrop-filter` en WebKitGTK: <https://linuxundich.de/en/webkitgtk-memory-leak-backdrop-filter/>
- librespot (alternativa de playback): <https://github.com/librespot-org/librespot>

---

## 8. Plan priorizado

### P0 — Esta semana (impacto alto, riesgo bajo)

**P0.1 — Publicar `.deb`/`.rpm` además del AppImage.**
- Portar las tareas `create:deb`/`create:rpm` y `build/linux/nfpm/nfpm.yaml` desde el scaffold de Wails v3: este repo reemplazó el Taskfile de Linux y hoy genera AppImage portable, AppImage lean y tarball.
- Declarar dependencias: `gtk4`, `webkitgtk-6.0`, `libsoup3`, `libmpg123`, `libFLAC`, `libvorbis`, `libogg`, `alsa-lib`.
- Impacto esperado: **115.5 MB → ~17 MB** para usuarios de distros modernas.
- Verificación: instalar en una VM limpia de Arch/Fedora/Ubuntu 24.04 y abrir la app.

**P0.2 — Convertir fuentes a WOFF2 y decidir Twemoji.**
- `woff2_compress frontend/public/fonts/JetBrainsMonoVariable.ttf` y cambiar el `@font-face` a `format("woff2-variations")`.
- Twemoji: WOFF2 (493 KB) o quitarlo si se acepta `Noto Color Emoji`.
- Impacto: **-1.16 MB** en disco/binario; **~-0.5 MB** en AppImage.
- Verificación: `du -sb frontend/dist/fonts`; revisar que los emojis de títulos se vean bien.

**P0.3 — Gatear animaciones decorativas y reducir blurs.**
- Añadir `.is-playing` a `aurora-drift`, `player-bloom`, `mark-breathe`, `provider-pulse`, `control-dot`, `breathe`, `ring-spin`; o envolverlas en `@media (prefers-reduced-motion: no-preference)`.
- Bajar `blur(30px)`/`blur(22px)` a 12-16 px; reemplazar `backdrop-filter` por fondos opacos donde el efecto sea sutil.
- Impacto: reducción medible de CPU/GPU en reposo; objetivo <10 % de un core con GPU.
- Verificación: protocolo de sección 9, antes/después.

**P0.4 — Polling del Web API: eventos primero, 1 s → adaptativo.**
- Reenviar `player_state_changed` desde `page.js` por el WebSocket de control y publicar el estado en Go; dejar `api.Player` como reconciliación cada 5-10 s.
- Mínimo: `webStreamPlayerInterval` a 2 s reproduciendo y 5 s pausado.
- Impacto: ~50-95 % menos requests; menos CPU/red/batería.

**P0.5 — Podar el runtime Electron.**
- Borrar `locales/*` salvo `en-US.pak` (y los idiomas soportados): **-8.5 MB**.
- Evaluar `libvk_swiftshader.so`: **-4.5 MB**.
- `LICENSES.chromium.html`: **-20 MB** solo si licencia lo permite; si no, comprimir.
- Impacto: **-13 a -33 MB** en disco por usuario.

### P1 — Próximas 2-4 semanas

**P1.1 — Mantener `web` como default y no depender de `librespot` para cuentas nuevas.**
- No cambiar `defaultPlayback()`: `web` sigue siendo el default del producto.
- `librespot` queda como opción avanzada para cuentas antiguas; documentar que las cuentas nuevas no funcionan por el cierre de endpoints (sección 12.2).
- El ahorro de runtime se logra con el browser del sistema (P1.5), no cambiando de backend.

**P1.2 — Arranque bajo demanda de go-librespot en modo web/Soloist.**
- Iniciar en la primera llamada de catálogo; detener tras N minutos sin uso.
- Impacto: **-28 MB PSS** y un proceso menos en reposo.
- Verificación: `GetState`/`Playlists` siguen funcionando tras el arranque diferido.

**P1.3 — Limpiar el perfil de developer.**
- Tras completar OAuth/onboarding, borrar `profile/WidevineCdm` y `profile/component_crx_cache` (o compartir perfil).
- Impacto: **-42 MB**.

**P1.4 — Ajustar caches.**
- Artwork: 64 MB → 32 MB de RAM; 256 MB → 128 MB de disco (o exponerlo).
- Letras: LRU/poda por antigüedad.
- Impacto: menos RAM/disco; bajo riesgo.

**P1.5 — Adapter de browser del sistema para el SDK (PoC en el repo).**
- Añadir un `SystemBrowser` detrás de un flag/setting: detectar `google-chrome`, `chromium`, `brave`, `microsoft-edge`; lanzar `--headless=new` con perfil dedicado; el page abre un WebSocket same-origin al control de Go y reporta `ready`/`error`/`player_state_changed`.
- Fallback automático a ECS si no hay browser compatible o el probe EME falla.
- Medir: PSS, CPU, tiempo de arranque, audio real, reconexión.
- Impacto esperado: **-283 MB** de runtime, **-47 MB** de perfil, **-117 MB** de descarga, **~-95 MB PSS** (262 vs 357 MB medidos). El AppImage no cambia.

**P1.6 — Hacer el browser del sistema el camino por defecto cuando esté disponible.**
- Mantener ECS solo como fallback descargable.
- Para Brave, setear `brave.widevine_opted_in=true` en el `Local State` del perfil antes del primer lanzamiento; para Chrome, no pasar `--disable-component-update`.

**P1.7 — AppImage lean (si se quiere mantener el formato portable).**
- Excluir `libwebkitgtk-6.0.so.4`, `libjavascriptcoregtk-6.0.so.1`, `libicu*`, helpers WebKit e `injected-bundle`; dejar que se resuelvan del sistema.
- No empaquetar `webkit2gtk-4.1`/GTK3 (ya se hace).
- Impacto: **~42 MB** con zstd-19 y bloque 1 MB.
- Verificación: VM con Arch/Fedora/Ubuntu 24.04 sin runtime propio; `ldd` debe resolver WebKit del sistema.

### P2 — Cuando haya tiempo

- **P2.1** Reemplazar el catálogo de go-librespot por Web API en modo web (elimina el sidecar por completo).
- **P2.2** Si se mantiene ECS como fallback, compartir perfil/componentes entre developer y webplayer.
- **P2.3** Visualizador a 15-20 fps / 22 kHz y opción de desactivarlo.
- **P2.4** Fondo sólido opcional (sin transparencia) para Wayland.
- **P2.5** Evaluar Flatpak/Flathub como canal principal.
- **P2.6** Reemplazar también el helper de developer (dashboard) por el browser del sistema y eliminar ECS por completo. Es el único camino para llegar a “cero Electron” en toda la app.

---

## 9. Protocolo de medición (para validar cada cambio)

1. **Tamaño:** `task clean && task package`; registrar `stat -c '%s'` de AppImage y tar.gz; `du -sh build/.../AppDir/usr/lib`.
2. **Memoria:** abrir la app en el modo a medir, esperar 30 s, y sumar PSS:
   ```bash
   for p in $(pgrep -x cadence) $(pgrep -f 'webkitgtk-6.0/WebKit') \
            $(pgrep -f 'cache/cadence/webplayer/runtime'); do
     awk '/^Pss:/{s+=$2} END{printf "%s %.0f MB\n", FILENAME, s/1024}' /proc/$p/smaps_rollup
   done
   ```
3. **CPU:** muestrear `/proc/<pid>/stat` (campos 14+15) durante 10 s en reposo y reproduciendo; repetir 3 veces y usar la mediana.
4. **Red:** contar requests del Web API con `--diagnostics` (`/debug/stats`) durante 5 minutos.
5. **Comparar siempre el mismo modo, tamaño de ventana y estado de reproducción.** El renderizado por software infla la CPU; usar el mismo entorno para el antes/después.

---

## 10. Qué NO hacer

- **No usar UPX** en `cadence`/`go-librespot`: reduce el archivo pero aumenta RAM y tiempo de arranque, y complica el debugging.
- **No borrar `libavif`/`libjxl`/`libgst*` del AppImage actual**: son `DT_NEEDED` de `libwebkitgtk`; la app no arranca. La única forma segura de quitarlos es no bundelear WebKit o rebuildearlo.
- **No confundir duplicados del AppDir con peso del AppImage**: squashfs los deduplica (medido: mismo tamaño exacto). El problema real es el contenido único.
- **No desactivar la aceleración por hardware** para “ahorrar”: en esta app empeora CPU y memoria.
- **No tocar `NO_STRIP=1` sin verificar el host**: en Arch no cambia nada; en otros hosts puede empeorar el tamaño.
- **No medir con la ventana tileada a 1280×720** y comparar contra 840×440: el área de repintado cambia los números.

---

## 11. Apéndice — Evidencia cruda

### A.1 Composición del binario

```
bin/cadence: 18,113,408 bytes, stripped
.text        7,009,253
.rodata      3,577,600   (incluye frontend/dist ~2.7 MB)
.gopclntab   5,447,320
.go.type     1,478,120
```

### A.2 AppDir

```
324 MB total
usr/lib  289 MB
usr/bin   34 MB  (cadence 16.5 + go-librespot 18.2)
usr/share 1.3 MB
157 archivos regulares, 0 symlinks
16.5 MB en copias duplicadas (deduplicadas por squashfs)
```

### A.3 Cache del usuario tras usar la app

```
~/.cache/cadence/webplayer/runtime   283 MB
~/.cache/cadence/webplayer/profile    47 MB  (Widevine 21 + crx 21)
~/.config/cadence/developer           59 MB  (Widevine 21 + crx 21)
~/.local/share/cadence/soloist        35 MB
```

### A.4 Procesos en modo web (reposo)

```
cadence              115 MB PSS
WebKit UI (4 proc)   225 MB PSS
Electron (9 proc)    357 MB PSS
go-librespot          28 MB PSS
TOTAL                725 MB PSS
```

### A.5 Perfil Go

```
heap inuse: 4,101.86 kB
goroutines: 18
eventos player:state: 35 en 17 s (~2/s)
```

### A.6 Conteos de CSS

```
animation: ... infinite   40
backdrop-filter           18
blur(...)                 42
@media prefers-reduced-motion  5 bloques
```

---


## 12. Reemplazo de Electron por un browser del sistema (investigación + PoC)

> Investigación posterior al análisis inicial. Responde a la pregunta: “¿se puede quitar Electron y seguir usando el SDK con un browser headless?”.

### 12.1 Qué se probó y qué dio

Se probó el peor caso técnico: **Widevine + audio real** en un Chromium headless, sin Electron.

Entorno: Brave 157.1.99.56 (Chromium 157.0.8103.50), Linux x86_64, PipeWire/PulseAudio, perfil dedicado en `/tmp`.

| Prueba | Resultado |
|---|---|
| `navigator.requestMediaKeySystemAccess('com.widevine.alpha', …)` en `--headless=new` | **WIDEVINE_OK** (audio `mp4a.40.2`, video `avc1.42E01E`, `SW_SECURE_CRYPTO`/`SW_SECURE_DECODE`) |
| `AudioContext` 440 Hz → `parec` del monitor de un sink nulo | **RMS 0.141, peak 0.200**: audio real, no silencio |
| PSS del árbol Brave con el page de audio (más el startup page de Brave) | **262 MB** (vs **357 MB** del helper Electron actual) |
| Descarga necesaria | **0 MB** (el browser ya está instalado) |
| Runtime en disco | **0 MB** (no se extrae nada) |

**Conclusión:** un Chromium del sistema en `--headless=new` puede hacer el trabajo de ECS: EME/Widevine y salida de audio al mixer del sistema. No es una hipótesis; quedó verificado en este equipo.

### 12.2 Por qué el SDK sigue siendo obligatorio (cuentas nuevas)

La investigación confirma la premisa del usuario:

- Spotify cerró los endpoints privados que usaba librespot para playback. El issue [librespot#1737](https://github.com/librespot-org/librespot/issues/1737) documenta `INVALID_CREDENTIALS` desde agosto de 2026; la comunidad lo atribuye al cierre previo al lanzamiento de Spotify Soloist.
- OwnTone documenta: “Spotify integration doesn't work if you have a Spotify account created after around November 2025” ([OwnTone](https://owntone.github.io/owntone-server/integrations/spotify/)).
- El **Web Playback SDK** es la vía soportada: requiere Premium, EME y Widevine; soporta Chrome, Firefox, Safari y Edge en desktop Linux ([docs oficiales](https://developer.spotify.com/documentation/web-playback-sdk)).
- Google lista Widevine como soportado en Chrome, Chromium, CEF/Electron y Firefox ([Widevine](https://developers.google.com/widevine/drm/overview)).

Por eso no se puede “simplificar” a librespot para cuentas nuevas. El SDK debe quedarse. La pregunta correcta no es *si* usar un browser, sino **qué browser** y **cuánto runtime propio** hace falta.

### 12.3 Arquitectura propuesta: browser-agnostic helper

El helper actual está acoplado a Electron en dos puntos:

1. `main.js` usa `BrowserWindow`, `components.whenReady()` (Widevine) y un WebSocket de control.
2. `page.js` usa `window.cadence` (preload) para reportar `ready`/`error`.

Para usar cualquier browser, se elimina el preload y el page habla directo con Go:

```
Browser (Chrome/Brave/Chromium/Firefox)
  └─ http://127.0.0.1:<port>/player?nonce=…
       ├─ fetch /token?nonce=…            (token de corta duración)
       └─ WebSocket ws://127.0.0.1:<port>/control?nonce=…
            → ready(device_id) / error(code,message) / player_state_changed
```

Go sigue siendo el dueño del ciclo de vida: lanza el proceso, espera `ready`, reinicia con backoff, mata el proceso al salir y sirve el page. El browser no recibe credenciales de Spotify; solo el token efímero, igual que hoy.

**Flags de lanzamiento (Chromium):**

```bash
google-chrome|chromium|brave \
  --headless=new \
  --user-data-dir="$PROFILE" \
  --autoplay-policy=no-user-gesture-required \
  --no-first-run --no-default-browser-check \
  --disable-sync --disable-extensions \
  "http://127.0.0.1:<port>/player?nonce=…"
```

Reglas:
- **No** pasar `--disable-component-update`: es lo que impide que Chrome registre Widevine en un perfil nuevo ([medición 2026](https://botbrowser.io/en/blog/widevine-cdm-availability-in-automated-browsers/)).
- **Brave** requiere `brave.widevine_opted_in=true` en `Local State` del perfil (verificado: sin eso, `NotSupportedError`; con eso, `WIDEVINE_OK`).
- **Chrome** trae el CDM; **Chromium** de distro a veces no. Antes de usarlo, correr un probe EME y, si falla, caer a ECS.
- **Firefox** está soportado por Spotify y por Widevine, pero necesita el CDM en el perfil (`media.gmp-widevinecdm.*`, `MOZ_GMP_PATH`) y el modo headless no está tan probado. Dejarlo como fallback experimental, no como camino principal.
- **Audio:** usar `--headless=new` (el viejo `chrome-headless-shell` no emite audio real). Verificado con `parec`.
- **Perfil:** dedicado y persistente; no reusar el perfil personal del usuario. Para Brave/Chrome, `--user-data-dir` crea una instancia separada; para Firefox, `--new-instance --profile`.

### 12.4 Matriz de decisión

| Opción | Disco | RAM (PSS) | Descarga | Riesgo | Veredicto |
|---|---:|---:|---:|---|---|
| ECS actual (Electron) | 283 MB + 47 MB | 357 MB | 117 MB | Bajo: funciona hoy | Mantener como fallback |
| Chrome/Brave/Chromium del sistema, `--headless=new` | 0 (ya instalado) | ~262 MB | 0 | Medio: browser, CDM, flags | **Camino principal** |
| Firefox headless | 0 (ya instalado) | ? | 0 | Alto: CDM y flags | Fallback experimental |
| CEF/CCS | ~150-200 MB | Similar a Chromium | ~100 MB | Alto: host C++ | No compensa |
| librespot sin browser | 0 | ~30 MB | 0 | **No viable en cuentas nuevas** | Descartado para web |
| ECS “lite” (podar locales/swiftshader) | 283 MB → ~250 MB | 357 MB | 117 MB | Bajo | Solo si no se adopta el browser del sistema |

**Sobre la RAM:** el browser del sistema no es mágicamente liviano; sigue siendo Chromium. La ganancia medida es ~95 MB PSS (262 vs 357 MB), no cientos de MB. La ganancia grande es **disco y descarga**: 330 MB menos en disco y 117 MB menos de descarga. Si el objetivo es RAM, la única vía es no usar Chromium (librespot/Soloist), que las cuentas nuevas no permiten.

### 12.5 Plan de implementación

**Fase A — PoC dentro del repo (1-2 días).**
- Nuevo paquete `internal/webplayer/systembrowser` con:
  - detección de binarios (`google-chrome`, `chromium`, `brave`, `microsoft-edge`);
  - probe EME vía CDP o vía el propio page (el page reporta `widevine_unavailable`);
  - lanzamiento con perfil dedicado y flags;
  - lifecycle (start/stop/restart, logs, timeout de `ready`).
- `page.js`: reemplazar `window.cadence` por un WebSocket same-origin; enviar `ready`, `error`, `player_state_changed`.
- `control.go`: aceptar conexiones de browser con nonce; el protocolo existente se reutiliza.
- Feature flag `CADENCE_WEB_HELPER=system|electron` (default `electron` durante el PoC).
- Criterio de aceptación: reproducir una playlist completa, pausar/reanudar, cambiar dispositivo, cerrar la app sin procesos huérfanos, y PSS < 300 MB.

**Fase B — Default con fallback (3-5 días).**
- Si hay browser compatible y el probe EME pasa, usar system browser; si no, descargar ECS como hoy.
- Medir en Arch/Fedora/Ubuntu 24.04 con Chrome, Brave y Chromium.
- Documentar la dependencia y el fallback en Settings.

**Fase C — Eliminar ECS del todo (opcional, más adelante).**
- Reemplazar también el helper de developer (dashboard) por el browser del sistema, o mover ese onboarding a un flujo CLI/URL.
- Solo entonces se puede borrar la descarga de ECS y el código de `components.whenReady()`.

### 12.6 Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| El usuario no tiene Chrome/Brave/Chromium | Fallback ECS; detectar y avisar en Settings |
| Chromium sin CDM | Probe EME; fallback ECS |
| Brave sin opt-in de Widevine | Escribir `brave.widevine_opted_in=true` en el perfil antes de lanzar |
| Flags de headless cambian entre versiones | Fijar flags mínimos; test de humo por versión |
| Audio silencioso en headless | Usar `--headless=new`; test de RMS con `parec` en CI local |
| Browser snap/flatpak no acepta `--user-data-dir` | Detectar y fallback |
| Procesos huérfanos | `Pdeathsig`/process group, igual que el helper actual |
| Perfil con cookies/datos | Perfil dedicado, permisos 0700, borrar en logout |
| Actualizaciones del CDM | Dejar que el browser las gestione; no fijar `--widevine-cdm-path` |

### 12.7 Fuentes de esta sección

- Spotify Web Playback SDK (browsers soportados, Premium, EME): <https://developer.spotify.com/documentation/web-playback-sdk>
- Spotify Web Player (browsers soportados en desktop Linux): <https://support.spotify.com/us/article/web-player-help/>
- OwnTone (cuentas creadas después de ~noviembre 2025): <https://owntone.github.io/owntone-server/integrations/spotify/>
- librespot#1737 (cierre de endpoints privados): <https://github.com/librespot-org/librespot/issues/1737>
- castlabs ECS (Widevine en Electron, component updater, `--widevine-cdm-path` no soportado): <https://github.com/castlabs/electron-releases>
- Widevine platforms (Chrome, Chromium, CEF/Electron, Firefox): <https://developers.google.com/widevine/drm/overview>
- Widevine en Chrome headless (component updater): <https://botbrowser.io/en/blog/widevine-cdm-availability-in-automated-browsers/>
- Chrome new headless (audio y modo): <https://developer.chrome.com/docs/automation-and-testing/headless-chrome-shell>
- Firefox + Widevine en Linux (CDM en perfil): <https://bugzilla.mozilla.org/show_bug.cgi?id=1679354>

### 12.8 Comandos del PoC

```bash
# 1) Sink nulo para verificar audio sin molestar al usuario
pactl load-module module-null-sink sink_name=cadence_test

# 2) Perfil dedicado + CDM (Brave: opt-in; Chrome: ya lo trae)
#    Brave:
python3 - <<'EOF'
import json
p="/tmp/brave-profile/Local State"
d=json.load(open(p)); d.setdefault("brave",{})["widevine_opted_in"]=True
json.dump(d,open(p,"w"))
EOF

# 3) Lanzar headless con el page local
brave --headless=new --disable-gpu --no-first-run \
  --no-default-browser-check --disable-sync \
  --autoplay-policy=no-user-gesture-required \
  --user-data-dir=/tmp/brave-profile \
  --remote-debugging-port=9223 \
  http://127.0.0.1:8899/audio.html

# 4) Verificar EME por CDP (Runtime.evaluate)
#    navigator.requestMediaKeySystemAccess('com.widevine.alpha', ...)
#    → WIDEVINE_OK

# 5) Verificar audio real
timeout 1.5s parec --device=cadence_test.monitor \
  --format=float32le --rate=48000 --channels=2 > monitor.raw
python3 - <<'EOF'
import struct, math
raw=open("monitor.raw","rb").read(); n=len(raw)//4
v=struct.unpack("<%df"%n, raw[:n*4])
print("rms", math.sqrt(sum(x*x for x in v)/len(v)), "peak", max(abs(x) for x in v))
EOF
```

Resultado medido: `WIDEVINE_OK`, `rms 0.14118`, `peak 0.20000`, PSS del árbol Brave **262 MB**.

## 13. Resultados de la implementación (2026-11-12)

Rama `perf/runtime-optimizations`. Todo lo siguiente está implementado y probado con `go test ./...` y `npm run check`/`build`.

| Cambio | Antes | Después | Ganancia |
|---|---:|---:|---:|
| PSS en reposo (web, sin reproducir) | 725 MB | **353 MB** | **-372 MB (-51 %)** |
| Helper Electron en reposo | 357 MB PSS | **0** (no se lanza) | -357 MB |
| go-librespot en reposo | 25 MB PSS | **0** (se lanza al abrir Browse) | -25 MB |
| AppImage portable | 115.5 MB | **106.5 MB** | **-9.0 MB** |
| AppImage lean (WebKit del sistema) | — | **62.7 MB** | **-43.8 MB vs portable** |
| Tarball | 16.2 MB | **14.28 MB** | **-1.9 MB** |
| Binario `cadence` | 18.1 MB | **16.97 MB** | **-1.14 MB** |
| Fuentes del frontend | 2.1 MB | **964 KB** | **-1.17 MB** |
| Runtime Electron (versiones viejas) | hasta 2 versiones | **1** | hasta -283 MB |
| Perfil developer (Widevine/crx) | 42 MB duplicados | **0** | -42 MB |

### Qué quedó implementado

- **Arranque lazy del helper**: `provisionWeb` instala y autentica, pero no lanza Electron. `EnsureStarted` espera el `ready` real (15 s en caliente, 2 min en frío si falta Widevine) y recién entonces el provider llama al Web API. El retry posterior es corto (3 × 500 ms) y solo cubre el device que todavía no aparece.
- **Apagado por inactividad**: a los 10 min en pausa, si el device SDK no es el activo, el helper se detiene. Nunca mientras el device SDK está activo. El toggle **Mantener Cadence Web disponible** (default apagado) desactiva el apagado y arranca el helper al inicio.
- **Push de estado**: `page.js` reenvía `player_state_changed` por el control WS; Go interpola la posición con `position + timestamp`. Mientras el device SDK está activo, el provider no llama `/me/player`; el polling queda como reconciliación y es adaptativo (3 s reproduciendo, 5 s pausado, 10 s detenido). `Retry-After` en 429 ya estaba implementado y sigue vigente.
- **Developer helper**: recibe `--disable-component-update`, no llama `components.whenReady()` y poda `WidevineCdm`/`component_crx_cache` al construirse.
- **Login OAuth**: usa el navegador del sistema (`xdg-open`), no el helper de Electron. El helper de developer queda solo para el setup único (Client ID / Soloist key).
- **Pdeathsig**: el proceso se lanza desde una goroutine con `runtime.LockOSThread()` que vive hasta que el helper termina.
- **Flags de memoria**: `--enable-low-end-device-mode`, `--js-flags=--max-old-space-size=64`, `--disable-features=MediaRouter,Translate,OptimizationHints`. Medición: 360 MB PSS (sin cambio significativo frente a 357 MB). `--disable-gpu` baja a **338 MB PSS (-22 MB)**, pero queda opt-in con `CADENCE_WEB_DISABLE_GPU=1` porque el ahorro es modesto y conviene medir CPU en cada equipo. Widevine siguió funcionando en ambos casos (el helper llegó a `ready`).
- **Caches**: artwork 64 → 32 MB de RAM y 256 → 128 MB de disco; letras con tope de 2000 archivos / 16 MB y poda de las más viejas.
- **Build**: fuentes WOFF2, screenshots fuera del tarball, AppImage con zstd-19 y bloque de 1 MB (appimagetool), y el runtime conserva una sola versión.

### Pendiente / próximos pasos

- **Smoke test de reproducción real** para el push de estado: la unidad está cubierta con un backend falso; falta reproducir una pista con el SDK real y verificar que `/me/player` deja de llamarse mientras el device SDK está activo.
- **AppImage lean**: implementado como segundo artefacto (`cadence-<version>-linux-x86_64-lean.AppImage`, 62.7 MB). Usa WebKitGTK 6.0 del sistema; requiere `gtk4`, `webkitgtk-6.0` y `libsoup3`. El `.deb`/`.rpm` (~17 MB) sigue pendiente.
- **Browser del sistema en lugar de Electron**: el PoC de la sección 12 sigue válido; no se implementó en esta rama.
- **`--disable-gpu`**: evaluar CPU en hardware real antes de hacerlo default.

**Nota final:** este documento es un diagnóstico con un plan de acción. Las P0 no cambian la arquitectura ni la experiencia; las P1 sí implican decisiones de producto (especialmente el reemplazo de Electron por el browser del sistema). Cualquier cambio de empaquetado debe validarse en una VM limpia de al menos Arch, Fedora y Ubuntu 24.04 antes de publicarse.

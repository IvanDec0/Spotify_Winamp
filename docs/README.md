# Documentation

| Document | Purpose |
|---|---|
| [`../documentacion-cliente-musical-skins.md`](../documentacion-cliente-musical-skins.md) | Original design baseline (historical architecture sections) |
| [`../plan/nombre_plan.md`](../plan/nombre_plan.md) | Implementation plan and test gates |
| [`../plan/cadence-v2.md`](../plan/cadence-v2.md) | v2 roadmap and plan-time baseline (implemented through Phase 8: release) |
| [`../CHANGELOG.md`](../CHANGELOG.md) | Release history |
| [`architecture.md`](architecture.md) | Current architecture source: Go package layout, dependency rule, event flow, audio backend seam and catalog extension points |
| [`adr/0004-dual-audio-backend.md`](adr/0004-dual-audio-backend.md) | ADR: Soloist as the default audio backend, go-librespot as the catalog engine and explicit fallback |
| [`i18n.md`](i18n.md) | Catalogs, key groups, ICU rules, locale sync, translation guard and backend error policy |
| [`development.md`](development.md) | Local setup, commands and tooling notes |
| [`installation.md`](installation.md) | Release installation, pairing and uninstall |
| [`spotify-setup.md`](spotify-setup.md) | Default Soloist backend, one-time API key, pairing, automatic updates and troubleshooting for both engines |
| [`archive/documentacion-cliente-musical-skins.es.md`](archive/documentacion-cliente-musical-skins.es.md) | Original Spanish design document (archived) |

[`architecture.md`](architecture.md) is the current source of truth for the
implemented architecture. The v2 plan is the roadmap and plan-time baseline;
Phase 8 (release) is the final phase. The original design document is kept for
visual and skin-system rationale; its Spotify Connect sections are historical.

The Spotify web player is the default playback backend; Cadence downloads its
Chromium runtime, opens the embedded login when no session is stored and starts
the helper automatically at startup. Soloist and go-librespot remain explicit
fallbacks, and go-librespot serves the library, search, context and catalog
session in every mode. Setup is documented in
[`spotify-setup.md`](spotify-setup.md#spotify-soloist-backend-optional-fallback);
the original decision and its trade-offs are in
[`adr/0004-dual-audio-backend.md`](adr/0004-dual-audio-backend.md).

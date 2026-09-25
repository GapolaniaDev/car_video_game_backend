# Specs Index — Racing Game Backend MVP

This folder contains the implementation specs derived from `../PROMPT.md`. Every spec was created on first pass, so all are marked `_pendiente.md` until implementation completes each one. When a spec is finished, rename to `_DONE.md`.

## Dependency Graph

```
01 → 02 → 03 → 04 ─┐
                ↓
        05 → 06 ─┐
                ↓
        07 ─────┼──→ 14 ──→ 15 ──→ 16
                ↓                   ↑
        08 → 09 ─┤                   │
                ↓                   │
        10 ─────┤                   │
                ↓                   │
        11 ─────┤                   │
                ↓                   │
        12 ─────┤                   │
                ↓                   │
        13 ─────┴───────────────────┘
```

## Specs

| # | File | Title | Status |
|---|------|-------|--------|
| 01 | `01_project_setup_and_go_module_DONE.md` | Project Setup & Go Module Init | **DONE** |
| 02 | `02_configuration_package_DONE.md` | Configuration Package | **DONE** |
| 03 | `03_database_infrastructure_DONE.md` | Database Infrastructure (Postgres + pgxpool) | **DONE** |
| 04 | `04_migrations_DONE.md` | Migrations | **DONE** |
| 05 | `05_rest_api_and_health_endpoint_DONE.md` | REST API + Health Endpoint | **DONE** |
| 06 | `06_api_dockerfile_DONE.md` | API Dockerfile | **DONE** |
| 07 | `07_nginx_configuration_DONE.md` | Nginx Configuration | **DONE** |
| 08 | `08_udp_game_server_ping_pong_DONE.md` | UDP Game Server (PING/PONG) | **DONE** |
| 09 | `09_game_server_dockerfile_DONE.md` | Game Server Dockerfile | **DONE** |
| 10 | `10_udp_test_client_DONE.md` | UDP Test Client | **DONE** |
| 11 | `11_protocol_buffers_DONE.md` | Protocol Buffers (`game.proto`) | **DONE** |
| 12 | `12_tests_DONE.md` | Tests | **DONE** |
| 13 | `13_graceful_shutdown_and_healthchecks_DONE.md` | Graceful Shutdown & Health Checks | **DONE** |
| 14 | `14_docker_compose_orchestration_DONE.md` | Docker Compose Orchestration | **DONE** |
| 15 | `15_readme_documentation_DONE.md` | README Documentation | **DONE** |
| 16 | `16_final_validation_and_verification_DONE.md` | Final Validation & Verification | **DONE** |

Final report: [`specs/REPORT.md`](REPORT.md).

## Recommended Execution Order

Specs are designed so that lower numbers are foundational. The clean implementation sequence mirrors section 36 of the prompt:

1. 01 → 02 → 03 → 04 (foundation)
2. 05 → 06 → 07 (REST path)
3. 08 → 09 → 10 (UDP path)
4. 11 (Protobuf contract — independent)
5. 12 (tests)
6. 13 (graceful shutdown + healthchecks)
7. 14 (compose orchestration — depends on everything above)
8. 15 (README — final docs)
9. 16 (run everything end-to-end and produce final report)

## Renaming Convention

When a spec's acceptance criteria pass:
1. Rename the file from `_pendiente.md` to `_DONE.md`.
2. Update the **Status:** line at the top to `DONE`.
3. Update this README's status cell.
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
| 01 | `01_project_setup_and_go_module_pendiente.md` | Project Setup & Go Module Init | pendiente |
| 02 | `02_configuration_package_pendiente.md` | Configuration Package | pendiente |
| 03 | `03_database_infrastructure_pendiente.md` | Database Infrastructure (Postgres + pgxpool) | pendiente |
| 04 | `04_migrations_pendiente.md` | Migrations | pendiente |
| 05 | `05_rest_api_and_health_endpoint_pendiente.md` | REST API + Health Endpoint | pendiente |
| 06 | `06_api_dockerfile_pendiente.md` | API Dockerfile | pendiente |
| 07 | `07_nginx_configuration_pendiente.md` | Nginx Configuration | pendiente |
| 08 | `08_udp_game_server_ping_pong_pendiente.md` | UDP Game Server (PING/PONG) | pendiente |
| 09 | `09_game_server_dockerfile_pendiente.md` | Game Server Dockerfile | pendiente |
| 10 | `10_udp_test_client_pendiente.md` | UDP Test Client | pendiente |
| 11 | `11_protocol_buffers_pendiente.md` | Protocol Buffers (`game.proto`) | pendiente |
| 12 | `12_tests_pendiente.md` | Tests | pendiente |
| 13 | `13_graceful_shutdown_and_healthchecks_pendiente.md` | Graceful Shutdown & Health Checks | pendiente |
| 14 | `14_docker_compose_orchestration_pendiente.md` | Docker Compose Orchestration | pendiente |
| 15 | `15_readme_documentation_pendiente.md` | README Documentation | pendiente |
| 16 | `16_final_validation_and_verification_pendiente.md` | Final Validation & Verification | pendiente |

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
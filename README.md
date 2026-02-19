# Agenda de Pacientes (Go + Cobra)

CLI para registrar pacientes y gestionar citas de consultorio con soporte multi-profesional y multi-servicio.

## Requisitos

- Go 1.26+

## Ejecutar

```bash
go run . --help
```

## Flags globales

- `--data-file`: ruta del archivo de datos (`.json` para backend JSON o `.db` para backend SQLite).
- `--storage`: backend de persistencia (`json` o `sqlite`). Default: `json`.
- `--actor`: actor responsable de la operacion. Es obligatorio para comandos mutables.

## Comandos principales

```bash
# Pacientes
go run . --actor "recepcion" patients add --name "Ana Perez" --phone "555-1234" --email "ana@mail.com"
go run . patients get --id "p_abc123"
go run . --actor "recepcion" patients update --id "p_abc123" --phone "555-9999"
go run . --actor "recepcion" patients delete --id "p_abc123"
go run . patients search --query "ana"
go run . patients list

# Profesionales
go run . --actor "admin" professionals add --name "Dr. Rivera" --primary-role "medico" --secondary-role "pediatra"
go run . professionals list
go run . professionals get --id "pr_abc123"
go run . --actor "admin" professionals update --id "pr_abc123" --secondary-role "clinica"
go run . --actor "admin" professionals delete --id "pr_abc123"
go run . professionals search --query "rivera"

# Servicios
go run . --actor "admin" services add --name "Consultorio 1" --kind "consultorio"
go run . services list
go run . services get --id "sv_abc123"
go run . --actor "admin" services update --id "sv_abc123" --kind "sala quirurgica"
go run . --actor "admin" services delete --id "sv_abc123"
go run . services search --query "quirurgica"

# Citas
go run . --actor "recepcion" appointments add \
  --patient-id "p_abc123" \
  --professional-id "pr_abc123" \
  --service-id "sv_abc123" \
  --at "2026-03-10 09:30" \
  --reason "Control anual"

go run . --actor "recepcion" appointments reschedule --id "a_def456" --at "2026-03-12 10:00"
go run . --actor "recepcion" appointments set-status --id "a_def456" --status "confirmada"
go run . appointments list
go run . appointments list --from "2026-03-01 00:00" --to "2026-03-31 23:59" --patient-id "p_abc123" --professional-id "pr_abc123" --service-id "sv_abc123" --status "programada"
go run . appointments list --all
go run . --actor "recepcion" appointments cancel --id "a_def456"
```

## Formularios interactivos en citas

Los comandos `appointments add`, `appointments reschedule` y `appointments cancel` soportan `-f` o `--form` para completar campos faltantes de forma interactiva.

```bash
go run . --actor "recepcion" appointments add -f --patient-id "p_abc123"
```

## Persistencia y migracion

### Backend JSON (default)

```bash
go run . --storage json --data-file ./agenda_data.json patients list
```

### Backend SQLite

```bash
go run . --storage sqlite --data-file ./agenda.db patients list
```

### Migrar JSON -> SQLite

```bash
go run . --actor "admin" storage migrate \
  --from-json ./agenda_data.json \
  --to-sqlite ./agenda.db

# Sobrescribir destino SQLite no vacio
go run . --actor "admin" storage migrate \
  --from-json ./agenda_data.json \
  --to-sqlite ./agenda.db \
  --force-overwrite
```

## Reglas de negocio

- Pacientes duplicados: se bloquean si coinciden por email (case-insensitive) o por telefono normalizado (solo digitos).
- Profesionales duplicados: se bloquean por nombre normalizado (lower + trim + espacios colapsados).
- Servicios duplicados: se bloquean por nombre normalizado.
- `patients update`, `professionals update`, `services update`: requieren `--id` y al menos un campo editable.
- `patients delete`: bloqueado si el paciente tiene citas activas (`programada` o `confirmada`).
- `professionals delete` y `services delete`: bloqueados si tienen citas activas.
- `appointments add`: requiere `patient-id`, `professional-id`, `service-id`, motivo no vacio y fecha futura.
- Fecha/hora sin zona (`YYYY-MM-DD HH:MM`): se interpreta en zona local del sistema.
- Conflictos de agenda: no se permiten dos citas activas en el mismo minuto con el mismo profesional o el mismo servicio.
- Filtros de fecha en `appointments list`: limites inclusivos (`--from` y `--to`).
- Estados de cita: `programada`, `confirmada`, `atendida`, `ausente`, `cancelada`.
- Transiciones validas:
  - `programada -> confirmada|ausente|cancelada`
  - `confirmada -> atendida|ausente|cancelada`
  - `atendida`, `ausente`, `cancelada` no transicionan.
- `reschedule` y `cancel` solo permiten citas en estado `programada` o `confirmada`.

## Robustez y auditoria

- JSON usa escritura atomica (`tmp + rename`) y sincronizacion de archivo.
- JSON genera backups rotativos (7 copias) antes de cada reemplazo exitoso.
- Auditoria append-only en `<data-file>.audit.log` (JSON Lines).
- Eventos auditados: altas/ediciones/bajas de pacientes, profesionales y servicios; operaciones de citas; migracion de storage; y backfill automatico v3.

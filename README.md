# Agenda de Pacientes (Go + Cobra)

CLI para registrar pacientes y gestionar citas de consultorio.

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
# Crear paciente (comando mutable: requiere --actor)
go run . --actor "recepcion" patients add --name "Ana Perez" --phone "555-1234" --email "ana@mail.com"

# Obtener un paciente por ID
go run . patients get --id "p_abc123"

# Actualizar parcialmente un paciente (mutable)
go run . --actor "recepcion" patients update --id "p_abc123" --phone "555-9999"

# Eliminar paciente (mutable, solo si no tiene citas activas)
go run . --actor "recepcion" patients delete --id "p_abc123"

# Buscar pacientes
go run . patients search --query "ana"

# Listar pacientes
go run . patients list

# Programar cita (mutable)
go run . --actor "recepcion" appointments add --patient-id "p_abc123" --at "2026-03-10 09:30" --reason "Control anual"

# Reprogramar cita (mutable)
go run . --actor "recepcion" appointments reschedule --id "a_def456" --at "2026-03-12 10:00"

# Cambiar estado de cita (mutable)
go run . --actor "recepcion" appointments set-status --id "a_def456" --status "confirmada"

# Listar citas activas
go run . appointments list

# Listar con filtros
go run . appointments list --from "2026-03-01 00:00" --to "2026-03-31 23:59" --patient-id "p_abc123" --status "programada"

# Incluir canceladas
go run . appointments list --all

# Cancelar cita (mutable)
go run . --actor "recepcion" appointments cancel --id "a_def456"
```

## Cuestionarios interactivos en citas

Los comandos `appointments add`, `appointments reschedule` y `appointments cancel` soportan `-q` o `--questionnaire` para completar campos faltantes de forma interactiva.

```bash
# Alta de cita usando cuestionario
# (si pasas --patient-id o --at se usan como valores preestablecidos)
go run . --actor "recepcion" appointments add -q --patient-id "p_abc123"
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
- `patients update`: requiere `--id` y al menos un campo editable (`--name`, `--phone`, `--email`).
- `patients delete`: bloqueado si el paciente tiene citas activas (`programada` o `confirmada`).
- `appointments add`: requiere motivo no vacio.
- Fecha/hora sin zona (`YYYY-MM-DD HH:MM`): se interpreta en zona local del sistema.
- Fechas de citas: deben ser futuras.
- Conflictos: no se permiten dos citas activas (`programada` o `confirmada`) en el mismo minuto.
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
- Eventos auditados: altas/ediciones/bajas de pacientes, operaciones de citas, y migracion de storage.

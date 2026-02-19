# Agenda de Pacientes (Go + Cobra)

CLI para registrar pacientes y agendar/cancelar citas.

## Requisitos

- Go 1.26+

## Ejecutar

```bash
go run . --help
```

## Comandos principales

```bash
# Crear paciente
go run . patients add --name "Ana Perez" --phone "555-1234" --email "ana@mail.com"

# Listar pacientes
go run . patients list

# Programar cita
go run . appointments add --patient-id "p_abc123" --at "2026-03-10 09:30" --reason "Control anual"

# Listar citas activas
go run . appointments list

# Incluir canceladas
go run . appointments list --all

# Cancelar cita
go run . appointments cancel --id "a_def456"
```

## Persistencia

Por defecto guarda datos en `agenda_data.json`. Puedes cambiarlo con:

```bash
go run . --data-file /ruta/mi_agenda.json patients list
```

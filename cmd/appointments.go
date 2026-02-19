package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"agenda-pacientes/internal/questionnaire"
	qauto "agenda-pacientes/internal/questionnaire/ui/auto"
	"agenda-pacientes/internal/questionnaire/ui/tty"
	"agenda-pacientes/internal/store"
	"github.com/spf13/cobra"
)

func newAppointmentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "appointments",
		Short: "Gestion de citas",
	}

	cmd.AddCommand(newAppointmentsAddCmd())
	cmd.AddCommand(newAppointmentsListCmd())
	cmd.AddCommand(newAppointmentsCancelCmd())
	cmd.AddCommand(newAppointmentsRescheduleCmd())
	cmd.AddCommand(newAppointmentsSetStatusCmd())

	return cmd
}

func newAppointmentsAddCmd() *cobra.Command {
	var patientID string
	var at string
	var reason string
	var useQuestionnaire bool

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Programar una cita",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}

			if useQuestionnaire {
				answers, err := collectQuestionnaireAnswers(cmd, appointmentAddForm(), questionnaire.Answers{
					"patient-id": strings.TrimSpace(patientID),
					"at":         strings.TrimSpace(at),
					"reason":     strings.TrimSpace(reason),
				})
				if err != nil {
					return err
				}
				patientID = answers["patient-id"]
				at = answers["at"]
				reason = answers["reason"]
			}

			if strings.TrimSpace(patientID) == "" {
				return fmt.Errorf("falta flag requerida: --patient-id")
			}
			if strings.TrimSpace(at) == "" {
				return fmt.Errorf("falta flag requerida: --at")
			}
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("reason es obligatorio")
			}

			dateTime, err := parseDateTime(at)
			if err != nil {
				return err
			}

			appt, err := appStore.ScheduleAppointment(patientID, dateTime, reason)
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Cita creada: %s | paciente: %s | fecha: %s\n",
				appt.ID,
				appt.PatientID,
				appt.DateTime.Format("2006-01-02 15:04"),
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&patientID, "patient-id", "", "ID del paciente")
	cmd.Flags().StringVar(&at, "at", "", "Fecha y hora (YYYY-MM-DD HH:MM o RFC3339)")
	cmd.Flags().StringVar(&reason, "reason", "", "Motivo de la consulta")
	addQuestionnaireFlags(cmd, &useQuestionnaire)

	return cmd
}

func newAppointmentsListCmd() *cobra.Command {
	var includeCanceled bool
	var fromValue string
	var toValue string
	var patientID string
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Listar citas",
		RunE: func(cmd *cobra.Command, args []string) error {
			var fromPtr *time.Time
			if strings.TrimSpace(fromValue) != "" {
				from, err := parseDateTime(fromValue)
				if err != nil {
					return fmt.Errorf("from invalido: %w", err)
				}
				fromPtr = &from
			}

			var toPtr *time.Time
			if strings.TrimSpace(toValue) != "" {
				to, err := parseDateTime(toValue)
				if err != nil {
					return fmt.Errorf("to invalido: %w", err)
				}
				toPtr = &to
			}

			if fromPtr != nil && toPtr != nil && fromPtr.After(*toPtr) {
				return fmt.Errorf("from no puede ser mayor que to")
			}

			filters := store.AppointmentFilters{
				IncludeCanceled: includeCanceled,
				From:            fromPtr,
				To:              toPtr,
				PatientID:       patientID,
				Status:          status,
			}

			appointments, err := appStore.ListAppointments(filters)
			if err != nil {
				return err
			}

			if len(appointments) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hay citas registradas.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Citas:")
			for _, a := range appointments {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"- %s | paciente: %s | fecha: %s | estado: %s | motivo: %s\n",
					a.ID,
					a.PatientID,
					a.DateTime.Format("2006-01-02 15:04"),
					a.Status,
					blankIfEmpty(a.Reason),
				)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&includeCanceled, "all", false, "Incluir citas canceladas")
	cmd.Flags().StringVar(&fromValue, "from", "", "Fecha/hora minima (inclusiva)")
	cmd.Flags().StringVar(&toValue, "to", "", "Fecha/hora maxima (inclusiva)")
	cmd.Flags().StringVar(&patientID, "patient-id", "", "Filtrar por ID de paciente")
	cmd.Flags().StringVar(&status, "status", "", "Filtrar por estado de cita")

	return cmd
}

func newAppointmentsCancelCmd() *cobra.Command {
	var appointmentID string
	var useQuestionnaire bool

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancelar una cita",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}

			if useQuestionnaire {
				answers, err := collectQuestionnaireAnswers(cmd, appointmentCancelForm(), questionnaire.Answers{
					"id": strings.TrimSpace(appointmentID),
				})
				if err != nil {
					return err
				}
				appointmentID = answers["id"]
			}

			if strings.TrimSpace(appointmentID) == "" {
				return fmt.Errorf("falta flag requerida: --id")
			}

			if err := appStore.CancelAppointment(appointmentID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cita cancelada: %s\n", appointmentID)
			return nil
		},
	}

	cmd.Flags().StringVar(&appointmentID, "id", "", "ID de la cita")
	addQuestionnaireFlags(cmd, &useQuestionnaire)
	return cmd
}

func newAppointmentsRescheduleCmd() *cobra.Command {
	var appointmentID string
	var at string
	var useQuestionnaire bool

	cmd := &cobra.Command{
		Use:   "reschedule",
		Short: "Reprogramar una cita",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}

			if useQuestionnaire {
				answers, err := collectQuestionnaireAnswers(cmd, appointmentRescheduleForm(), questionnaire.Answers{
					"id": strings.TrimSpace(appointmentID),
					"at": strings.TrimSpace(at),
				})
				if err != nil {
					return err
				}
				appointmentID = answers["id"]
				at = answers["at"]
			}

			if strings.TrimSpace(appointmentID) == "" {
				return fmt.Errorf("falta flag requerida: --id")
			}
			if strings.TrimSpace(at) == "" {
				return fmt.Errorf("falta flag requerida: --at")
			}

			dateTime, err := parseDateTime(at)
			if err != nil {
				return err
			}

			appt, err := appStore.RescheduleAppointment(appointmentID, dateTime)
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Cita reprogramada: %s | paciente: %s | nueva fecha: %s\n",
				appt.ID,
				appt.PatientID,
				appt.DateTime.Format("2006-01-02 15:04"),
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&appointmentID, "id", "", "ID de la cita")
	cmd.Flags().StringVar(&at, "at", "", "Nueva fecha y hora (YYYY-MM-DD HH:MM o RFC3339)")
	addQuestionnaireFlags(cmd, &useQuestionnaire)

	return cmd
}

func newAppointmentsSetStatusCmd() *cobra.Command {
	var appointmentID string
	var status string

	cmd := &cobra.Command{
		Use:   "set-status",
		Short: "Actualizar estado de una cita",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if strings.TrimSpace(status) == "" {
				return fmt.Errorf("falta flag requerida: --status")
			}
			if strings.TrimSpace(appointmentID) == "" {
				return fmt.Errorf("falta flag requerida: --id")
			}

			appt, err := appStore.SetAppointmentStatus(appointmentID, status)
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Estado actualizado: %s | cita: %s | estado: %s\n",
				appt.PatientID,
				appt.ID,
				appt.Status,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&appointmentID, "id", "", "ID de la cita")
	cmd.Flags().StringVar(&status, "status", "", "Nuevo estado (programada|confirmada|atendida|ausente|cancelada)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("status")
	return cmd
}

func addQuestionnaireFlags(cmd *cobra.Command, enabled *bool) {
	cmd.Flags().BoolVar(enabled, "questionnaire", false, "Habilita captura interactiva de campos")
	cmd.Flags().BoolVar(enabled, "q", false, "Alias corto de --questionnaire")
}

func appointmentAddForm() questionnaire.FormSpec {
	return questionnaire.FormSpec{
		CommandKey: "appointments.add",
		Fields: []questionnaire.FieldSpec{
			{
				Name:     "patient-id",
				Label:    "ID del paciente",
				Required: true,
				Type:     questionnaire.FieldTypeID,
			},
			{
				Name:     "at",
				Label:    "Fecha y hora",
				Required: true,
				Type:     questionnaire.FieldTypeDateTime,
				Help:     "Formato: YYYY-MM-DD HH:MM o RFC3339",
				Validate: func(value string) error {
					_, err := parseDateTime(value)
					return err
				},
			},
			{
				Name:     "reason",
				Label:    "Motivo",
				Required: true,
				Type:     questionnaire.FieldTypeString,
			},
		},
	}
}

func appointmentRescheduleForm() questionnaire.FormSpec {
	return questionnaire.FormSpec{
		CommandKey: "appointments.reschedule",
		Fields: []questionnaire.FieldSpec{
			{
				Name:     "id",
				Label:    "ID de la cita",
				Required: true,
				Type:     questionnaire.FieldTypeID,
			},
			{
				Name:     "at",
				Label:    "Nueva fecha y hora",
				Required: true,
				Type:     questionnaire.FieldTypeDateTime,
				Help:     "Formato: YYYY-MM-DD HH:MM o RFC3339",
				Validate: func(value string) error {
					_, err := parseDateTime(value)
					return err
				},
			},
		},
	}
}

func appointmentCancelForm() questionnaire.FormSpec {
	return questionnaire.FormSpec{
		CommandKey: "appointments.cancel",
		Fields: []questionnaire.FieldSpec{
			{
				Name:     "id",
				Label:    "ID de la cita",
				Required: true,
				Type:     questionnaire.FieldTypeID,
			},
		},
	}
}

func collectQuestionnaireAnswers(cmd *cobra.Command, form questionnaire.FormSpec, preset questionnaire.Answers) (questionnaire.Answers, error) {
	for k, v := range preset {
		if strings.TrimSpace(v) == "" {
			delete(preset, k)
		}
	}

	renderer := qauto.New(&tty.Renderer{
		In:  os.Stdin,
		Out: cmd.OutOrStdout(),
	})
	engine := questionnaire.Engine{Renderer: renderer}
	answers, err := engine.Collect(context.Background(), form, preset)
	if err != nil {
		if errors.Is(err, questionnaire.ErrCanceled) {
			return nil, fmt.Errorf("cuestionario cancelado")
		}
		return nil, err
	}
	return answers, nil
}

func parseDateTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("fecha invalida: valor vacio")
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04", trimmed, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("fecha invalida: usa 'YYYY-MM-DD HH:MM' o RFC3339")
}

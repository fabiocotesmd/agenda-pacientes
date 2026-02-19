package cmd

import (
	"fmt"
	"strings"
	"time"

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

	return cmd
}

func newAppointmentsAddCmd() *cobra.Command {
	var patientID string
	var at string
	var reason string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Programar una cita",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(patientID) == "" {
				return fmt.Errorf("patient-id es obligatorio")
			}
			if strings.TrimSpace(at) == "" {
				return fmt.Errorf("at es obligatorio")
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
	_ = cmd.MarkFlagRequired("patient-id")
	_ = cmd.MarkFlagRequired("at")

	return cmd
}

func newAppointmentsListCmd() *cobra.Command {
	var includeCanceled bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Listar citas",
		RunE: func(cmd *cobra.Command, args []string) error {
			appointments, err := appStore.ListAppointments(includeCanceled)
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
	return cmd
}

func newAppointmentsCancelCmd() *cobra.Command {
	var appointmentID string

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancelar una cita",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(appointmentID) == "" {
				return fmt.Errorf("id es obligatorio")
			}
			if err := appStore.CancelAppointment(appointmentID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cita cancelada: %s\n", appointmentID)
			return nil
		},
	}

	cmd.Flags().StringVar(&appointmentID, "id", "", "ID de la cita")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func parseDateTime(value string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04",
		time.RFC3339,
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("fecha invalida: usa 'YYYY-MM-DD HH:MM' o RFC3339")
}

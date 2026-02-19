package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPatientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patients",
		Short: "Gestion de pacientes",
	}

	cmd.AddCommand(newPatientsAddCmd())
	cmd.AddCommand(newPatientsListCmd())
	cmd.AddCommand(newPatientsGetCmd())
	cmd.AddCommand(newPatientsUpdateCmd())
	cmd.AddCommand(newPatientsDeleteCmd())
	cmd.AddCommand(newPatientsSearchCmd())

	return cmd
}

func newPatientsAddCmd() *cobra.Command {
	var name string
	var phone string
	var email string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Registrar un paciente",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("el nombre es obligatorio")
			}

			patient, err := appStore.AddPatient(name, phone, email)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Paciente creado: %s (%s)\n", patient.Name, patient.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Nombre completo del paciente")
	cmd.Flags().StringVar(&phone, "phone", "", "Telefono")
	cmd.Flags().StringVar(&email, "email", "", "Correo electronico")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func newPatientsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar pacientes",
		RunE: func(cmd *cobra.Command, args []string) error {
			patients, err := appStore.ListPatients()
			if err != nil {
				return err
			}

			if len(patients) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hay pacientes registrados.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Pacientes:")
			for _, p := range patients {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s | %s | tel: %s | email: %s\n", p.ID, p.Name, blankIfEmpty(p.Phone), blankIfEmpty(p.Email))
			}
			return nil
		},
	}
}

func newPatientsGetCmd() *cobra.Command {
	var patientID string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Ver detalle de un paciente",
		RunE: func(cmd *cobra.Command, args []string) error {
			patient, err := appStore.GetPatientByID(patientID)
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Paciente: %s | %s | tel: %s | email: %s\n",
				patient.ID,
				patient.Name,
				blankIfEmpty(patient.Phone),
				blankIfEmpty(patient.Email),
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&patientID, "id", "", "ID del paciente")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newPatientsUpdateCmd() *cobra.Command {
	var patientID string
	var name string
	var phone string
	var email string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Actualizar datos de un paciente",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" && strings.TrimSpace(phone) == "" && strings.TrimSpace(email) == "" {
				return fmt.Errorf("debe proporcionar al menos un campo para actualizar")
			}

			patient, err := appStore.UpdatePatient(patientID, name, phone, email)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Paciente actualizado: %s (%s)\n", patient.Name, patient.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&patientID, "id", "", "ID del paciente")
	cmd.Flags().StringVar(&name, "name", "", "Nuevo nombre")
	cmd.Flags().StringVar(&phone, "phone", "", "Nuevo telefono")
	cmd.Flags().StringVar(&email, "email", "", "Nuevo correo electronico")
	_ = cmd.MarkFlagRequired("id")

	return cmd
}

func newPatientsDeleteCmd() *cobra.Command {
	var patientID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Eliminar un paciente",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if err := appStore.DeletePatient(patientID); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Paciente eliminado: %s\n", patientID)
			return nil
		},
	}

	cmd.Flags().StringVar(&patientID, "id", "", "ID del paciente")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newPatientsSearchCmd() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Buscar pacientes por nombre, telefono o email",
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := appStore.SearchPatients(query)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No se encontraron pacientes.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Pacientes encontrados:")
			for _, p := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s | %s | tel: %s | email: %s\n", p.ID, p.Name, blankIfEmpty(p.Phone), blankIfEmpty(p.Email))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Texto de busqueda")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

func blankIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

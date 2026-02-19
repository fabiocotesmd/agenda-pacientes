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

func blankIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

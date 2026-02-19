package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newProfessionalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "professionals",
		Short: "Gestion de profesionales",
	}

	cmd.AddCommand(newProfessionalsAddCmd())
	cmd.AddCommand(newProfessionalsListCmd())
	cmd.AddCommand(newProfessionalsGetCmd())
	cmd.AddCommand(newProfessionalsUpdateCmd())
	cmd.AddCommand(newProfessionalsDeleteCmd())
	cmd.AddCommand(newProfessionalsSearchCmd())

	return cmd
}

func newProfessionalsAddCmd() *cobra.Command {
	var name string
	var primaryRole string
	var secondaryRole string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Registrar un profesional",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("el nombre del profesional es obligatorio")
			}
			if strings.TrimSpace(primaryRole) == "" {
				return fmt.Errorf("primary-role es obligatorio")
			}

			professional, err := appStore.AddProfessional(name, primaryRole, secondaryRole)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Profesional creado: %s (%s)\n", professional.Name, professional.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Nombre del profesional")
	cmd.Flags().StringVar(&primaryRole, "primary-role", "", "Rol primario")
	cmd.Flags().StringVar(&secondaryRole, "secondary-role", "", "Rol secundario")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("primary-role")
	return cmd
}

func newProfessionalsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar profesionales",
		RunE: func(cmd *cobra.Command, args []string) error {
			professionals, err := appStore.ListProfessionals()
			if err != nil {
				return err
			}

			if len(professionals) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hay profesionales registrados.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Profesionales:")
			for _, p := range professionals {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"- %s | %s | rol primario: %s | rol secundario: %s\n",
					p.ID,
					p.Name,
					blankIfEmpty(p.PrimaryRole),
					blankIfEmpty(p.SecondaryRole),
				)
			}
			return nil
		},
	}
}

func newProfessionalsGetCmd() *cobra.Command {
	var professionalID string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Ver detalle de un profesional",
		RunE: func(cmd *cobra.Command, args []string) error {
			professional, err := appStore.GetProfessionalByID(professionalID)
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Profesional: %s | %s | rol primario: %s | rol secundario: %s\n",
				professional.ID,
				professional.Name,
				blankIfEmpty(professional.PrimaryRole),
				blankIfEmpty(professional.SecondaryRole),
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&professionalID, "id", "", "ID del profesional")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newProfessionalsUpdateCmd() *cobra.Command {
	var professionalID string
	var name string
	var primaryRole string
	var secondaryRole string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Actualizar datos de un profesional",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" && strings.TrimSpace(primaryRole) == "" && strings.TrimSpace(secondaryRole) == "" {
				return fmt.Errorf("debe proporcionar al menos un campo para actualizar")
			}

			professional, err := appStore.UpdateProfessional(professionalID, name, primaryRole, secondaryRole)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Profesional actualizado: %s (%s)\n", professional.Name, professional.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&professionalID, "id", "", "ID del profesional")
	cmd.Flags().StringVar(&name, "name", "", "Nuevo nombre")
	cmd.Flags().StringVar(&primaryRole, "primary-role", "", "Nuevo rol primario")
	cmd.Flags().StringVar(&secondaryRole, "secondary-role", "", "Nuevo rol secundario")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newProfessionalsDeleteCmd() *cobra.Command {
	var professionalID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Eliminar un profesional",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if err := appStore.DeleteProfessional(professionalID); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Profesional eliminado: %s\n", professionalID)
			return nil
		},
	}

	cmd.Flags().StringVar(&professionalID, "id", "", "ID del profesional")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newProfessionalsSearchCmd() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Buscar profesionales por nombre o rol",
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := appStore.SearchProfessionals(query)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No se encontraron profesionales.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Profesionales encontrados:")
			for _, p := range results {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"- %s | %s | rol primario: %s | rol secundario: %s\n",
					p.ID,
					p.Name,
					blankIfEmpty(p.PrimaryRole),
					blankIfEmpty(p.SecondaryRole),
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Texto de busqueda")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

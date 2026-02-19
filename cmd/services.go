package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newServicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "Gestion de servicios",
	}

	cmd.AddCommand(newServicesAddCmd())
	cmd.AddCommand(newServicesListCmd())
	cmd.AddCommand(newServicesGetCmd())
	cmd.AddCommand(newServicesUpdateCmd())
	cmd.AddCommand(newServicesDeleteCmd())
	cmd.AddCommand(newServicesSearchCmd())

	return cmd
}

func newServicesAddCmd() *cobra.Command {
	var name string
	var kind string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Registrar un servicio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("el nombre del servicio es obligatorio")
			}
			if strings.TrimSpace(kind) == "" {
				return fmt.Errorf("kind es obligatorio")
			}

			service, err := appStore.AddService(name, kind)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Servicio creado: %s (%s)\n", service.Name, service.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Nombre del servicio")
	cmd.Flags().StringVar(&kind, "kind", "", "Tipo de servicio")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("kind")
	return cmd
}

func newServicesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar servicios",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := appStore.ListServices()
			if err != nil {
				return err
			}

			if len(services) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hay servicios registrados.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Servicios:")
			for _, s := range services {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s | %s | tipo: %s\n", s.ID, s.Name, blankIfEmpty(s.Kind))
			}
			return nil
		},
	}
}

func newServicesGetCmd() *cobra.Command {
	var serviceID string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Ver detalle de un servicio",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := appStore.GetServiceByID(serviceID)
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Servicio: %s | %s | tipo: %s\n",
				service.ID,
				service.Name,
				blankIfEmpty(service.Kind),
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceID, "id", "", "ID del servicio")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newServicesUpdateCmd() *cobra.Command {
	var serviceID string
	var name string
	var kind string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Actualizar datos de un servicio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" && strings.TrimSpace(kind) == "" {
				return fmt.Errorf("debe proporcionar al menos un campo para actualizar")
			}

			service, err := appStore.UpdateService(serviceID, name, kind)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Servicio actualizado: %s (%s)\n", service.Name, service.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceID, "id", "", "ID del servicio")
	cmd.Flags().StringVar(&name, "name", "", "Nuevo nombre")
	cmd.Flags().StringVar(&kind, "kind", "", "Nuevo tipo")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newServicesDeleteCmd() *cobra.Command {
	var serviceID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Eliminar un servicio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}
			if err := appStore.DeleteService(serviceID); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Servicio eliminado: %s\n", serviceID)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceID, "id", "", "ID del servicio")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newServicesSearchCmd() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Buscar servicios por nombre o tipo",
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := appStore.SearchServices(query)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No se encontraron servicios.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Servicios encontrados:")
			for _, s := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s | %s | tipo: %s\n", s.ID, s.Name, blankIfEmpty(s.Kind))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Texto de busqueda")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

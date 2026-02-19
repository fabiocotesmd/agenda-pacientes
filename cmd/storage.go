package cmd

import (
	"fmt"

	"agenda-pacientes/internal/store"
	"github.com/spf13/cobra"
)

func newStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Operaciones de persistencia y migracion",
	}

	cmd.AddCommand(newStorageMigrateCmd())
	return cmd
}

func newStorageMigrateCmd() *cobra.Command {
	var fromJSON string
	var toSQLite string
	var forceOverwrite bool

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrar datos desde JSON hacia SQLite",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireActorFlag(); err != nil {
				return err
			}

			result, err := store.MigrateJSONToSQLite(store.MigrationOptions{
				FromJSON:       fromJSON,
				ToSQLite:       toSQLite,
				Actor:          actor,
				ForceOverwrite: forceOverwrite,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Migracion completada | pacientes: %d | citas: %d | destino: %s\n",
				result.Patients,
				result.Appointments,
				toSQLite,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromJSON, "from-json", "", "Ruta del archivo JSON origen")
	cmd.Flags().StringVar(&toSQLite, "to-sqlite", "", "Ruta del archivo SQLite destino")
	cmd.Flags().BoolVar(&forceOverwrite, "force-overwrite", false, "Sobrescribe el destino SQLite si contiene datos")
	_ = cmd.MarkFlagRequired("from-json")
	_ = cmd.MarkFlagRequired("to-sqlite")

	return cmd
}

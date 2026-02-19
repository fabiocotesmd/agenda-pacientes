package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"agenda-pacientes/internal/store"
	"github.com/spf13/cobra"
)

var (
	dataFile    string
	storageType string
	actor       string
	appStore    *store.Store
)

var rootCmd = &cobra.Command{
	Use:   "agenda",
	Short: "Agenda de pacientes para consultorio",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		appStore = store.NewWithConfig(store.Config{
			Storage:         storageType,
			DataFile:        dataFile,
			Actor:           actor,
			BackupRetention: 7,
		})
		return appStore.InitError()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func requireActorFlag() error {
	if strings.TrimSpace(actor) == "" {
		return errors.New("actor es obligatorio: usa --actor \"nombre\"")
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataFile, "data-file", "agenda_data.json", "Ruta del archivo de datos (.json o .db)")
	rootCmd.PersistentFlags().StringVar(&storageType, "storage", "json", "Backend de persistencia: json o sqlite")
	rootCmd.PersistentFlags().StringVar(&actor, "actor", "", "Actor responsable de mutaciones")
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SilenceUsage = true

	rootCmd.AddCommand(newPatientsCmd())
	rootCmd.AddCommand(newProfessionalsCmd())
	rootCmd.AddCommand(newServicesCmd())
	rootCmd.AddCommand(newAppointmentsCmd())
	rootCmd.AddCommand(newStorageCmd())

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Muestra la version de la aplicacion",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "agenda v3.0.0")
		},
	})
}

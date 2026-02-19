package cmd

import (
	"fmt"
	"os"

	"agenda-pacientes/internal/store"
	"github.com/spf13/cobra"
)

var (
	dataFile string
	appStore *store.Store
)

var rootCmd = &cobra.Command{
	Use:   "agenda",
	Short: "Agenda de pacientes para consultorio",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		appStore = store.New(dataFile)
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataFile, "data-file", "agenda_data.json", "Ruta del archivo JSON de datos")
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SilenceUsage = true

	rootCmd.AddCommand(newPatientsCmd())
	rootCmd.AddCommand(newAppointmentsCmd())

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Muestra la version de la aplicacion",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "agenda v1.0.0")
		},
	})
}

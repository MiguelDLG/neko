package draw

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	Enabled bool
	// maximum number of persisted strokes kept on the board; oldest are dropped
	MaxStrokes int
	// maximum number of points a single stroke may have
	MaxPoints int
	// only admins may clear the whole board
	ClearAdminOnly bool
}

func (Config) Init(cmd *cobra.Command) error {
	cmd.PersistentFlags().Bool("draw.enabled", true, "whether to enable the shared drawing board plugin")
	if err := viper.BindPFlag("draw.enabled", cmd.PersistentFlags().Lookup("draw.enabled")); err != nil {
		return err
	}

	cmd.PersistentFlags().Int("draw.max_strokes", 2000, "maximum number of strokes kept on the drawing board")
	if err := viper.BindPFlag("draw.max_strokes", cmd.PersistentFlags().Lookup("draw.max_strokes")); err != nil {
		return err
	}

	cmd.PersistentFlags().Int("draw.max_points", 4000, "maximum number of points in a single stroke")
	if err := viper.BindPFlag("draw.max_points", cmd.PersistentFlags().Lookup("draw.max_points")); err != nil {
		return err
	}

	cmd.PersistentFlags().Bool("draw.clear_admin_only", false, "whether only admins can clear the drawing board")
	if err := viper.BindPFlag("draw.clear_admin_only", cmd.PersistentFlags().Lookup("draw.clear_admin_only")); err != nil {
		return err
	}

	return nil
}

func (s *Config) Set() {
	s.Enabled = viper.GetBool("draw.enabled")
	s.MaxStrokes = viper.GetInt("draw.max_strokes")
	s.MaxPoints = viper.GetInt("draw.max_points")
	s.ClearAdminOnly = viper.GetBool("draw.clear_admin_only")
}

package service

import (
	"fmt"
	"github.com/inovacc/daemon/internal/parameters"

	"github.com/inovacc/config"
	"github.com/spf13/cobra"
)

func Handler(_ *cobra.Command, _ []string) error {
	// Get the loaded configuration with type safety
	cfg, err := config.GetServiceConfig[*parameters.Service]()
	if err != nil {
		return fmt.Errorf("failed to get service config: %w", err)
	}

	fmt.Printf("Service running on %s:%d\n", cfg.Host, cfg.Port)

	// Access base configuration
	baseCfg := config.GetBaseConfig()
	fmt.Printf("Application ID: %s\n", baseCfg.AppID)

	return nil
}

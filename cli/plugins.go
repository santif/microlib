package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// PluginManager manages CLI plugins
type PluginManager struct {
	config *CLIConfig
}

// NewPluginManager creates a new plugin manager
func NewPluginManager() *PluginManager {
	return &PluginManager{
		config: GetConfig(),
	}
}

// LoadPlugins loads and registers all enabled plugins
func (pm *PluginManager) LoadPlugins(rootCmd *cobra.Command) error {
	for _, plugin := range pm.config.Plugins {
		if !plugin.Enabled {
			continue
		}

		if err := pm.registerPlugin(rootCmd, plugin); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load plugin %s: %v\n", plugin.Name, err)
		}
	}

	return nil
}

// registerPlugin registers a single plugin as a cobra command
func (pm *PluginManager) registerPlugin(rootCmd *cobra.Command, plugin PluginConfig) error {
	cmd := &cobra.Command{
		Use:   plugin.Name,
		Short: plugin.Description,
		Long:  fmt.Sprintf("Plugin: %s\n%s", plugin.Name, plugin.Description),
		RunE: func(cmd *cobra.Command, args []string) error {
			return pm.executePlugin(plugin, args)
		},
	}

	rootCmd.AddCommand(cmd)
	return nil
}

// executePlugin executes a plugin command
func (pm *PluginManager) executePlugin(plugin PluginConfig, args []string) error {
	// Check if plugin command exists
	cmdPath, err := exec.LookPath(plugin.Command)
	if err != nil {
		return fmt.Errorf("plugin command not found: %s", plugin.Command)
	}

	// Prepare environment variables from plugin config
	env := os.Environ()
	for key, value := range plugin.Config {
		env = append(env, fmt.Sprintf("MICROLIB_%s=%s", strings.ToUpper(key), value))
	}

	// Execute plugin
	cmd := exec.Command(cmdPath, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// InstallPlugin installs a new plugin
func (pm *PluginManager) InstallPlugin(name, command, description string) error {
	// Check if plugin already exists
	for _, plugin := range pm.config.Plugins {
		if plugin.Name == name {
			return fmt.Errorf("plugin %s already exists", name)
		}
	}

	// Verify command exists
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("command not found: %s", command)
	}

	// Add plugin to configuration
	newPlugin := PluginConfig{
		Name:        name,
		Command:     command,
		Description: description,
		Enabled:     true,
		Config:      make(map[string]string),
	}

	return UpdateConfig(func(config *CLIConfig) {
		config.Plugins = append(config.Plugins, newPlugin)
	})
}

// UninstallPlugin removes a plugin
func (pm *PluginManager) UninstallPlugin(name string) error {
	return UpdateConfig(func(config *CLIConfig) {
		for i, plugin := range config.Plugins {
			if plugin.Name == name {
				config.Plugins = append(config.Plugins[:i], config.Plugins[i+1:]...)
				return
			}
		}
	})
}

// EnablePlugin enables a plugin
func (pm *PluginManager) EnablePlugin(name string) error {
	return UpdateConfig(func(config *CLIConfig) {
		for i, plugin := range config.Plugins {
			if plugin.Name == name {
				config.Plugins[i].Enabled = true
				return
			}
		}
	})
}

// DisablePlugin disables a plugin
func (pm *PluginManager) DisablePlugin(name string) error {
	return UpdateConfig(func(config *CLIConfig) {
		for i, plugin := range config.Plugins {
			if plugin.Name == name {
				config.Plugins[i].Enabled = false
				return
			}
		}
	})
}

// ListPlugins returns all configured plugins
func (pm *PluginManager) ListPlugins() []PluginConfig {
	return pm.config.Plugins
}

// DiscoverPlugins discovers plugins in the system PATH
func (pm *PluginManager) DiscoverPlugins() ([]string, error) {
	var plugins []string

	// Get PATH directories
	pathEnv := os.Getenv("PATH")
	pathDirs := strings.Split(pathEnv, string(os.PathListSeparator))

	// Look for microlib-* executables
	for _, dir := range pathDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if strings.HasPrefix(name, "microlib-") && name != "microlib-cli" {
				pluginName := strings.TrimPrefix(name, "microlib-")
				plugins = append(plugins, pluginName)
			}
		}
	}

	return plugins, nil
}

// init initializes plugin commands
func init() {
	pluginManager := NewPluginManager()

	// Plugin management commands
	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage CLI plugins",
		Long:  `Commands for installing, managing, and using CLI plugins.`,
	}

	// Install plugin command
	installCmd := &cobra.Command{
		Use:   "install [name] [command]",
		Short: "Install a new plugin",
		Long:  `Install a new CLI plugin by specifying its name and command.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			command := args[1]
			description := fmt.Sprintf("Plugin: %s", name)

			if err := pluginManager.InstallPlugin(name, command, description); err != nil {
				return err
			}

			fmt.Printf("Plugin '%s' installed successfully\n", name)
			return nil
		},
	}

	// Uninstall plugin command
	uninstallCmd := &cobra.Command{
		Use:   "uninstall [name]",
		Short: "Uninstall a plugin",
		Long:  `Uninstall a CLI plugin by name.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := pluginManager.UninstallPlugin(name); err != nil {
				return err
			}

			fmt.Printf("Plugin '%s' uninstalled successfully\n", name)
			return nil
		},
	}

	// List plugins command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Long:  `List all installed CLI plugins and their status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins := pluginManager.ListPlugins()

			if len(plugins) == 0 {
				fmt.Println("No plugins installed")
				return nil
			}

			fmt.Println("Installed plugins:")
			fmt.Printf("%-15s | %-10s | %s\n", "Name", "Status", "Description")
			fmt.Println(strings.Repeat("-", 60))

			for _, plugin := range plugins {
				status := "disabled"
				if plugin.Enabled {
					status = "enabled"
				}
				fmt.Printf("%-15s | %-10s | %s\n", plugin.Name, status, plugin.Description)
			}

			return nil
		},
	}

	// Discover plugins command
	discoverCmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover available plugins",
		Long:  `Discover plugins available in the system PATH.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := pluginManager.DiscoverPlugins()
			if err != nil {
				return err
			}

			if len(plugins) == 0 {
				fmt.Println("No plugins discovered")
				return nil
			}

			fmt.Println("Discovered plugins:")
			for _, plugin := range plugins {
				fmt.Printf("  - %s (use 'microlib plugin install %s microlib-%s' to install)\n",
					plugin, plugin, plugin)
			}

			return nil
		},
	}

	// Enable plugin command
	enableCmd := &cobra.Command{
		Use:   "enable [name]",
		Short: "Enable a plugin",
		Long:  `Enable a previously installed plugin.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := pluginManager.EnablePlugin(name); err != nil {
				return err
			}

			fmt.Printf("Plugin '%s' enabled successfully\n", name)
			return nil
		},
	}

	// Disable plugin command
	disableCmd := &cobra.Command{
		Use:   "disable [name]",
		Short: "Disable a plugin",
		Long:  `Disable an installed plugin without uninstalling it.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := pluginManager.DisablePlugin(name); err != nil {
				return err
			}

			fmt.Printf("Plugin '%s' disabled successfully\n", name)
			return nil
		},
	}

	// Add subcommands
	pluginCmd.AddCommand(installCmd)
	pluginCmd.AddCommand(uninstallCmd)
	pluginCmd.AddCommand(listCmd)
	pluginCmd.AddCommand(discoverCmd)
	pluginCmd.AddCommand(enableCmd)
	pluginCmd.AddCommand(disableCmd)

	rootCmd.AddCommand(pluginCmd)

	// Load plugins into root command
	pluginManager.LoadPlugins(rootCmd)
}

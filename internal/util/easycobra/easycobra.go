package easycobra

import "github.com/spf13/cobra"

type Command struct {
	Use                   string
	Short                 string
	Aliases               []string
	Args                  cobra.PositionalArgs
	InitPersistentPreRunE func(cmd *cobra.Command) func(cmd *cobra.Command, args []string) error
	Run                   func(cmd *cobra.Command, args []string)
	InitRun               func(cmd *cobra.Command) func(cmd *cobra.Command, args []string)
	// private
	cmd *cobra.Command
}

func (c *Command) CobraCommand() *cobra.Command {
	if c.cmd != nil {
		return c.cmd
	}

	c.cmd = &cobra.Command{
		Use:     c.Use,
		Short:   c.Short,
		Aliases: c.Aliases,
		Args:    c.Args,
		Run:     c.Run,
	}
	if c.InitPersistentPreRunE != nil {
		c.cmd.PersistentPreRunE = c.InitPersistentPreRunE(c.cmd)
	}
	if c.InitRun != nil {
		c.cmd.Run = c.InitRun(c.cmd)
	}

	return c.cmd
}

func (c *Command) AddCommand(cmds ...*Command) {
	for _, cmd := range cmds {
		c.CobraCommand().AddCommand(cmd.CobraCommand())
	}
}

package easycobra

import (
	"github.com/spf13/cobra"
)

type Run func(args []string) error

type Command struct {
	Use      string
	Short    string
	Aliases  []string
	Args     cobra.PositionalArgs
	Hidden   bool // 透传给 cobra.Command.Hidden：为 true 时不显示在 help 等输出里
	Run      Run
	InitRun  func(cmd *cobra.Command) Run
	Children []*Command
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
		Hidden:  c.Hidden,
	}

	var run Run
	if c.InitRun != nil {
		run = c.InitRun(c.cmd)
	} else {
		run = c.Run
	}
	if run != nil {
		c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return run(args)
		}
	}

	for _, child := range c.Children {
		c.cmd.AddCommand(child.CobraCommand())
	}

	return c.cmd
}

func (c *Command) Execute() error {
	return c.CobraCommand().Execute()
}

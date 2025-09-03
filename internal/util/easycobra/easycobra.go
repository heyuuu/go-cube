package easycobra

import "github.com/spf13/cobra"

func AddCommand[T1, T2 any](root *Command[T1], sub *Command[T2]) {
	root.CobraCommand().AddCommand(sub.CobraCommand())
}

type Command[T any] struct {
	Use     string
	Short   string
	Aliases []string
	Args    cobra.PositionalArgs
	Init    func(cmd *cobra.Command, flags *T)
	Run     func(cmd *cobra.Command, flags *T, args []string)
	// private
	cmd *cobra.Command
}

func (c *Command[T]) CobraCommand() *cobra.Command {
	if c.cmd != nil {
		return c.cmd
	}

	c.cmd = &cobra.Command{
		Use:     c.Use,
		Short:   c.Short,
		Aliases: c.Aliases,
		Args:    c.Args,
	}

	var flags T
	if c.Run != nil {
		c.cmd.Run = func(cmd *cobra.Command, args []string) {
			c.Run(cmd, &flags, args)
		}
	}
	if c.Init != nil {
		c.Init(c.cmd, &flags)
	}

	return c.cmd
}
